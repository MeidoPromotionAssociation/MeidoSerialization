package KCES

import (
	"fmt"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
)

// ExpandedKCESPreset is the fully decoded editing representation of a KCES
// VirtualDirectory preset. Unlike KCESPreset, the three MaidPresetCore byte
// strings are represented by their actual game structures instead of base64
// blobs. Container metadata and unknown future slots remain available so the
// surrounding wire can still be written with its original versions.
type ExpandedKCESPreset struct {
	Format                  string                                 `json:"format"`
	ContainerVersion        int                                    `json:"containerVersion"`
	ContainerVersionless    bool                                   `json:"containerVersionless,omitempty"`
	ContainerFilesOnly      bool                                   `json:"containerFilesOnly,omitempty"`
	ContainerDirectoriesNil bool                                   `json:"containerDirectoriesNil,omitempty"`
	ContainerFilesNil       bool                                   `json:"containerFilesNil,omitempty"`
	ContainerFieldCount     *int                                   `json:"containerFieldCount,omitempty"`
	ContainerFutureSlots    [][]byte                               `json:"containerFutureSlots,omitempty"`
	ContainerDirectories    map[string]ct.VirtualDirectoryMetadata `json:"containerDirectories,omitempty"`
	ContainerVirtualFiles   map[string]ct.VirtualFileMetadata      `json:"containerVirtualFiles,omitempty"`
	Thumbnail               []byte                                 `json:"thumbnail"`
	MaidData                *ExpandedKCESPresetCore                `json:"maidData"`
	Meta                    *KCESPresetMeta                        `json:"meta,omitempty"`
	ExtraFiles              map[string][]byte                      `json:"extraFiles,omitempty"`
}

// ExpandedKCESPresetCore is MaidPresetCore with propData, colorData, and
// bodyData decoded into their respective BinaryWriter structures.
type ExpandedKCESPresetCore struct {
	*IndexedObjectMetadata
	Version      int                     `json:"version"`
	PropData     *KCESPresetPropertyList `json:"propData"`
	ColorData    *KCESPresetColorData    `json:"colorData"`
	BodyData     *KCESPresetBodyData     `json:"bodyData"`
	RootNil      bool                    `json:"rootNil,omitempty"`
	TrailingData []byte                  `json:"trailingData,omitempty"`
}

// DecodeExpandedKCESPreset decodes both the VirtualDirectory/MessagePack
// envelope and all three known MaidPresetCore byte-string payloads.
func DecodeExpandedKCESPreset(data []byte) (*ExpandedKCESPreset, error) {
	preset, err := DecodeKCESPreset(data)
	if err != nil {
		return nil, err
	}
	return ExpandKCESPreset(preset)
}

// ExpandKCESPreset converts an already-decoded opaque KCESPreset into its
// fully typed editing representation. Nil inner byte strings remain nil; a
// present but malformed block is rejected instead of being exposed as an
// unexplained byte array.
func ExpandKCESPreset(preset *KCESPreset) (*ExpandedKCESPreset, error) {
	if preset == nil {
		return nil, fmt.Errorf("nil KCES preset")
	}

	expanded := &ExpandedKCESPreset{
		Format:                  preset.Format,
		ContainerVersion:        preset.ContainerVersion,
		ContainerVersionless:    preset.ContainerVersionless,
		ContainerFilesOnly:      preset.ContainerFilesOnly,
		ContainerDirectoriesNil: preset.ContainerDirectoriesNil,
		ContainerFilesNil:       preset.ContainerFilesNil,
		ContainerFieldCount:     preset.ContainerFieldCount,
		ContainerFutureSlots:    preset.ContainerFutureSlots,
		ContainerDirectories:    preset.ContainerDirectories,
		ContainerVirtualFiles:   preset.ContainerVirtualFiles,
		Thumbnail:               preset.Thumbnail,
		Meta:                    preset.Meta,
		ExtraFiles:              preset.ExtraFiles,
	}
	if preset.MaidData == nil {
		return nil, fmt.Errorf("KCES preset maidData is required")
	}

	core := &ExpandedKCESPresetCore{
		IndexedObjectMetadata: preset.MaidData.IndexedObjectMetadata,
		Version:               preset.MaidData.Version,
		RootNil:               preset.MaidData.RootNil,
		TrailingData:          preset.MaidData.TrailingData,
	}
	var err error
	if preset.MaidData.PropData != nil {
		core.PropData, err = DecodeKCESPresetPropertyData(preset.MaidData.PropData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.propData: %w", err)
		}
	}
	if preset.MaidData.ColorData != nil {
		core.ColorData, err = DecodeKCESPresetColorData(preset.MaidData.ColorData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.colorData: %w", err)
		}
	}
	if preset.MaidData.BodyData != nil {
		core.BodyData, err = DecodeKCESPresetBodyData(preset.MaidData.BodyData)
		if err != nil {
			return nil, fmt.Errorf("decode KCES preset maidData.bodyData: %w", err)
		}
	}
	expanded.MaidData = core
	return expanded, nil
}

// EncodeExpandedKCESPreset rebuilds the three inner BinaryWriter blocks and
// then writes the ordinary KCES preset envelope. Every stored version comes
// from value; no game migration callback or current-version constructor is
// invoked.
func EncodeExpandedKCESPreset(value *ExpandedKCESPreset) ([]byte, error) {
	preset, err := CollapseExpandedKCESPreset(value)
	if err != nil {
		return nil, err
	}
	return EncodeKCESPreset(preset)
}

// CollapseExpandedKCESPreset converts the editing representation back to the
// byte-string form required by MaidPresetCore's MessagePack wire layout.
func CollapseExpandedKCESPreset(value *ExpandedKCESPreset) (*KCESPreset, error) {
	if value == nil {
		return nil, fmt.Errorf("nil expanded KCES preset")
	}
	if value.MaidData == nil {
		return nil, fmt.Errorf("expanded KCES preset maidData is required")
	}

	core := &KCESPresetCore{
		IndexedObjectMetadata: value.MaidData.IndexedObjectMetadata,
		Version:               value.MaidData.Version,
		RootNil:               value.MaidData.RootNil,
		TrailingData:          value.MaidData.TrailingData,
	}
	var err error
	if value.MaidData.PropData != nil {
		core.PropData, err = EncodeKCESPresetPropertyData(value.MaidData.PropData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.propData: %w", err)
		}
	}
	if value.MaidData.ColorData != nil {
		core.ColorData, err = EncodeKCESPresetColorData(value.MaidData.ColorData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.colorData: %w", err)
		}
	}
	if value.MaidData.BodyData != nil {
		core.BodyData, err = EncodeKCESPresetBodyData(value.MaidData.BodyData)
		if err != nil {
			return nil, fmt.Errorf("encode expanded KCES preset maidData.bodyData: %w", err)
		}
	}

	return &KCESPreset{
		Format:                  value.Format,
		ContainerVersion:        value.ContainerVersion,
		ContainerVersionless:    value.ContainerVersionless,
		ContainerFilesOnly:      value.ContainerFilesOnly,
		ContainerDirectoriesNil: value.ContainerDirectoriesNil,
		ContainerFilesNil:       value.ContainerFilesNil,
		ContainerFieldCount:     value.ContainerFieldCount,
		ContainerFutureSlots:    value.ContainerFutureSlots,
		ContainerDirectories:    value.ContainerDirectories,
		ContainerVirtualFiles:   value.ContainerVirtualFiles,
		Thumbnail:               value.Thumbnail,
		MaidData:                core,
		Meta:                    value.Meta,
		ExtraFiles:              value.ExtraFiles,
	}, nil
}
