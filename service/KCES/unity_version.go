package KCES

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultKCESUnityVersion = "2022.3.35f1"
	// defaultKCESTargetPlatform 是官方 KCES 文件统一使用的 Windows Standalone 64 位平台 ID
	// defaultKCESTargetPlatform is the 64-bit Windows Standalone platform ID used uniformly by official KCES files
	defaultKCESTargetPlatform      uint32 = 19
	defaultKCESGenerationVersion          = "5.x.x"
	supportedSerializedFileVersion uint32 = 22
	defaultKCESAbaVersion          uint32 = 8
	minimumCanonicalUnityMajor     int64  = 2020
	minimumCanonicalUnityMinor     int64  = 2
)

// unityPackSettings 保存纯目录打包唯一允许的 Unity 写入契约 / unityPackSettings stores the sole Unity writing contract allowed for pure-directory packing
type unityPackSettings struct {
	UnityVersion          string // SerializedFile Unity 版本 / SerializedFile Unity version
	EngineVersion         string // UnityFS 引擎版本 / UnityFS engine version
	TargetPlatform        uint32 // Unity 目标平台 / Unity target platform
	AbaVersion            uint32 // UnityFS 格式版本 / UnityFS format version
	GenerationVersion     string // UnityFS generation 版本 / UnityFS generation version
	SerializedFileVersion uint32 // SerializedFile 格式版本 / SerializedFile format version
}

// resolveUnityPackSettings 返回 KCES 纯目录打包使用的固定 Unity 契约
// resolveUnityPackSettings returns the fixed Unity contract used by KCES pure-directory packing
func resolveUnityPackSettings() unityPackSettings {
	return unityPackSettings{
		UnityVersion:          defaultKCESUnityVersion,
		EngineVersion:         defaultKCESUnityVersion,
		TargetPlatform:        defaultKCESTargetPlatform,
		AbaVersion:            defaultKCESAbaVersion,
		GenerationVersion:     defaultKCESGenerationVersion,
		SerializedFileVersion: supportedSerializedFileVersion,
	}
}

// validateCanonicalSourceUnityVersion 确认源 Unity 版本不早于 KCES 已知使用的 2020.2 版本线并对后续版本保持开放
// validateCanonicalSourceUnityVersion confirms that the source Unity version is no earlier than the 2020.2 line known to be used by KCES while remaining open to later versions
func validateCanonicalSourceUnityVersion(version string) error {
	major, minor, err := parseUnityMajorMinor(version)
	if err != nil {
		return fmt.Errorf("invalid source Unity version %q: %w", version, err)
	}
	if major < minimumCanonicalUnityMajor || major == minimumCanonicalUnityMajor && minor < minimumCanonicalUnityMinor {
		return fmt.Errorf("unsupported source Unity version %q: pure-directory ABA unpacking requires Unity %d.%d or later", version, minimumCanonicalUnityMajor, minimumCanonicalUnityMinor)
	}
	return nil
}

// parseUnityMajorMinor 将 Unity 版本的 major.minor 前缀解析为固定宽度整数
// parseUnityMajorMinor parses a Unity version's major.minor prefix into fixed-width integers
func parseUnityMajorMinor(version string) (int64, int64, error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("expected major.minor version")
	}
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || major <= 0 {
		return 0, 0, fmt.Errorf("invalid major component %q", parts[0])
	}
	minor, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || minor < 0 {
		return 0, 0, fmt.Errorf("invalid minor component %q", parts[1])
	}
	return major, minor, nil
}

// readAssetMetaStrict 读取单个原始对象编辑接口使用的 metadata 文件，缺失时返回零值
// readAssetMetaStrict reads the metadata file used by the individual raw-object editing API and returns a zero value when it is absent
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
