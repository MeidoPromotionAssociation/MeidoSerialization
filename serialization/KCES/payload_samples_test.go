package KCES

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeKCESPayload_FromTestdataSamples(t *testing.T) {
	pathsByExt := groupPayloadSamplesByExt(t)
	for ext, paths := range pathsByExt {
		ext := ext
		paths := paths
		t.Run(ext, func(t *testing.T) {
			for _, path := range paths {
				path := path
				t.Run(filepath.Base(path), func(t *testing.T) {
					assertPayloadSampleRoundTripDeepEqual(t, path)
				})
			}
		})
	}
}

func groupPayloadSamplesByExt(t *testing.T) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(payloadSampleDir(), "*"))
	if err != nil {
		t.Fatalf("glob payload samples: %v", err)
	}
	if len(paths) == 0 {
		t.Skip("no payload samples found in testdata/kces_payload")
	}

	pathsByExt := map[string][]string{}
	for _, path := range paths {
		name := filepath.Base(path)
		ext := NormalizeKCESPayloadExtension(name)
		if ext == "" {
			t.Fatalf("unexpected payload sample %q", name)
		}
		if kind := payloadKindForExtension(ext); kind == "" {
			t.Fatalf("no payload kind for suffix %q sample %q", ext, name)
		}
		pathsByExt[ext] = append(pathsByExt[ext], path)
	}
	return pathsByExt
}

func assertPayloadSampleRoundTripDeepEqual(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload sample %s: %v", path, err)
	}
	env, err := DecodeKCESPayload(data, name)
	if err != nil {
		t.Fatalf("DecodeKCESPayload: %v", err)
	}
	assertPayloadEnvelopeStrict(t, env, name)

	encoded, err := EncodeKCESPayload(env)
	if err != nil {
		t.Fatalf("EncodeKCESPayload: %v", err)
	}
	decoded, err := DecodeKCESPayload(encoded, name)
	if err != nil {
		t.Fatalf("re-decode %s: %v", name, err)
	}
	if !reflect.DeepEqual(decoded, env) {
		t.Fatalf("%s changed after decode/encode/decode: got %#v, want %#v", name, decoded, env)
	}
}

func assertPayloadEnvelopeStrict(t *testing.T, env *KCESPayloadEnvelope, name string) {
	t.Helper()
	ext := NormalizeKCESPayloadExtension(name)
	if env.Format != PayloadFormatKCESMessagePack {
		t.Fatalf("format got %q, want %q", env.Format, PayloadFormatKCESMessagePack)
	}
	if env.Extension != ext {
		t.Fatalf("extension got %q, want %q", env.Extension, ext)
	}
	wantKind := payloadKindForExtension(ext)
	if env.Kind != wantKind {
		t.Fatalf("kind got %q, want %q", env.Kind, wantKind)
	}
	if !env.LengthPrefixed {
		t.Fatalf("expected %s to be length-prefixed", name)
	}

	switch wantKind {
	case PayloadKindDynamicBoneStatus:
		if env.DynamicBone == nil || env.DynamicBone.Version == 0 {
			t.Fatalf("missing dynamicBoneStatus: %+v", env)
		}
	case PayloadKindJSONString:
		if env.Text == "" || len(env.JSON) == 0 {
			t.Fatalf("missing JSON string payload: %+v", env)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(env.Text)); err != nil {
			t.Fatalf("text field is not valid JSON: %v", err)
		}
		if !bytes.Equal(env.JSON, compact.Bytes()) {
			t.Fatalf("JSON string fields are not consistent: text=%q json=%s", env.Text, env.JSON)
		}
	case PayloadKindColliderPackage:
		if env.ColliderPackage == nil || env.ColliderPackage.Version == 0 || len(env.ColliderPackage.Colliders) == 0 {
			t.Fatalf("missing colliderPackage: %+v", env)
		}
	case PayloadKindLimbCollider:
		if env.LimbCollider == nil || env.LimbCollider.Version == 0 || len(env.LimbCollider.Items) == 0 {
			t.Fatalf("missing limbColliderPackage: %+v", env)
		}
	case PayloadKindIKCollider:
		if env.IKCollider == nil || env.IKCollider.Version == 0 || len(env.IKCollider.Groups) == 0 {
			t.Fatalf("missing ikColliderPackage: %+v", env)
		}
	case PayloadKindClothParams:
		if env.ClothParams == nil {
			t.Fatalf("missing clothParams: %+v", env)
		}
	case PayloadKindRawMsgpack:
		if env.MsgpackBase64 == "" {
			t.Fatalf("missing raw msgpack payload")
		}
	default:
		t.Fatalf("unsupported payload kind %q", wantKind)
	}
}

func readPayloadSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(payloadSampleDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read payload sample %s: %v", path, err)
	}
	return data
}

func payloadSampleDir() string {
	return filepath.Join("..", "..", "testdata", "kces_payload")
}
