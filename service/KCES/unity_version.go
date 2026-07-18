package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultKCESUnityVersion               = "2021.3.37f1"
	defaultKCESTargetPlatform      uint32 = 5 // Preserves the legacy writer default when sidecar context is unavailable
	defaultKCESGenerationVersion          = "5.x.x"
	supportedSerializedFileVersion        = 22
)

// unityPackSettings is the single version contract shared by the generated
// SerializedFile and its containing UnityFS .aba file.
type unityPackSettings struct {
	UnityVersion          string
	EngineVersion         string
	TargetPlatform        uint32
	AbaVersion            uint32
	GenerationVersion     string
	SerializedFileVersion uint32
}

// resolveUnityPackSettings merges version context from every asset sidecar.
// Missing fields do not participate in conflict detection, preserving support
// for sidecars created by older MeidoSerialization releases.
func resolveUnityPackSettings(metas []rawAssetMeta, sources []string) (unityPackSettings, error) {
	var settings unityPackSettings
	var unitySource, engineSource, targetSource, abaSource, generationSource, serializedSource string
	var targetSet, abaSet, serializedSet bool

	for i, meta := range metas {
		source := fmt.Sprintf("asset[%d]", i)
		if i < len(sources) && sources[i] != "" {
			source = sources[i]
		}

		if err := mergeStringSetting("unityVersion", &settings.UnityVersion, &unitySource, meta.UnityVersion, source); err != nil {
			return unityPackSettings{}, err
		}
		if err := mergeStringSetting("engineVersion", &settings.EngineVersion, &engineSource, meta.EngineVersion, source); err != nil {
			return unityPackSettings{}, err
		}
		if meta.TargetPlatform != nil {
			if err := mergeUint32Setting("targetPlatform", &settings.TargetPlatform, &targetSet, &targetSource, *meta.TargetPlatform, source); err != nil {
				return unityPackSettings{}, err
			}
		}
		if meta.AbaVersion != 0 {
			if err := mergeUint32Setting("abaVersion", &settings.AbaVersion, &abaSet, &abaSource, meta.AbaVersion, source); err != nil {
				return unityPackSettings{}, err
			}
		}
		if err := mergeStringSetting("generationVersion", &settings.GenerationVersion, &generationSource, meta.GenerationVersion, source); err != nil {
			return unityPackSettings{}, err
		}
		if meta.SerializedFileVersion != 0 {
			if err := mergeUint32Setting("serializedFileVersion", &settings.SerializedFileVersion, &serializedSet, &serializedSource, meta.SerializedFileVersion, source); err != nil {
				return unityPackSettings{}, err
			}
		}
	}

	// A source may only have one of these legacy/new fields. Treat either one as
	// authoritative, but never emit different UnityFS and SerializedFile engine
	// versions because Unity interprets native object bytes using that contract.
	switch {
	case settings.UnityVersion == "" && settings.EngineVersion == "":
		settings.UnityVersion = defaultKCESUnityVersion
		settings.EngineVersion = defaultKCESUnityVersion
	case settings.UnityVersion == "":
		settings.UnityVersion = settings.EngineVersion
	case settings.EngineVersion == "":
		settings.EngineVersion = settings.UnityVersion
	case settings.UnityVersion != settings.EngineVersion:
		return unityPackSettings{}, fmt.Errorf(
			"Unity version contract conflict: unityVersion %q from %s differs from engineVersion %q from %s",
			settings.UnityVersion, unitySource, settings.EngineVersion, engineSource,
		)
	}

	major, minor, err := parseUnityMajorMinor(settings.UnityVersion)
	if err != nil {
		return unityPackSettings{}, fmt.Errorf("invalid unityVersion %q: %w", settings.UnityVersion, err)
	}
	if !targetSet {
		settings.TargetPlatform = defaultKCESTargetPlatform
	}
	if !abaSet {
		settings.AbaVersion = abaVersionForUnity(major, minor)
	}
	if settings.AbaVersion != 7 && settings.AbaVersion != 8 {
		return unityPackSettings{}, fmt.Errorf(
			"unsupported abaVersion %d from %s (KCES packing supports UnityFS versions 7 and 8)",
			settings.AbaVersion, settingSource(abaSource),
		)
	}
	if settings.GenerationVersion == "" {
		settings.GenerationVersion = defaultKCESGenerationVersion
	}
	if !serializedSet {
		settings.SerializedFileVersion = supportedSerializedFileVersion
	}
	if settings.SerializedFileVersion != supportedSerializedFileVersion {
		return unityPackSettings{}, fmt.Errorf(
			"unsupported serializedFileVersion %d from %s (writer emits version %d)",
			settings.SerializedFileVersion, settingSource(serializedSource), supportedSerializedFileVersion,
		)
	}

	return settings, nil
}

func mergeStringSetting(field string, dst *string, dstSource *string, candidate string, source string) error {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil
	}
	if strings.ContainsRune(candidate, '\x00') {
		return fmt.Errorf("invalid %s from %s: contains NUL", field, source)
	}
	if *dst == "" {
		*dst = candidate
		*dstSource = source
		return nil
	}
	if *dst != candidate {
		return fmt.Errorf("conflicting %s sidecars: %q from %s, %q from %s", field, *dst, *dstSource, candidate, source)
	}
	return nil
}

func mergeUint32Setting(field string, dst *uint32, set *bool, dstSource *string, candidate uint32, source string) error {
	if !*set {
		*dst = candidate
		*set = true
		*dstSource = source
		return nil
	}
	if *dst != candidate {
		return fmt.Errorf("conflicting %s sidecars: %d from %s, %d from %s", field, *dst, *dstSource, candidate, source)
	}
	return nil
}

func parseUnityMajorMinor(version string) (int, int, error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("expected major.minor version")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major <= 0 {
		return 0, 0, fmt.Errorf("invalid major component %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, fmt.Errorf("invalid minor component %q", parts[1])
	}
	return major, minor, nil
}

func abaVersionForUnity(major int, minor int) uint32 {
	// KCES samples built with 2020.2/2021.3 use UnityFS v7; 2022.3 uses v8.
	// This fallback covers those observed families when an older sidecar has no
	// explicit abaVersion; newly unpacked sidecars preserve the exact value.
	if major > 2022 || (major == 2022 && minor >= 2) {
		return 8
	}
	return 7
}

func settingSource(source string) string {
	if source == "" {
		return "derived/default settings"
	}
	return source
}

func readAssetMetaStrict(assetPath string) (rawAssetMeta, error) {
	data, err := os.ReadFile(assetMetaPath(assetPath))
	if err != nil {
		if os.IsNotExist(err) {
			return rawAssetMeta{}, nil
		}
		return rawAssetMeta{}, fmt.Errorf("read asset metadata %q: %w", assetMetaPath(assetPath), err)
	}
	var meta rawAssetMeta
	if err := json.Unmarshal(trimJSONUTF8BOM(data), &meta); err != nil {
		return rawAssetMeta{}, fmt.Errorf("parse asset metadata %q: %w", assetMetaPath(assetPath), err)
	}
	return meta, nil
}
