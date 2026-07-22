package knowledgev1

// kcesContainerProfiles contains the VirtualDirectory-backed KCES formats and
// the system-data container. These profiles deliberately distinguish game
// meaningful virtual files from container metadata that must only be preserved.
func kcesContainerProfiles() map[string]Guide {
	vdSource := source("KCES 1.34.4", "game/KCES 1.34.4/WfSystem.Serialization/VirtualDirectory.cs", "VirtualDirectory.Serialize/Deserialize", 409, 575, "VirtualDirectory writes a fixed signature, a serialize-type byte, raw virtual-file data, compressed indexed metadata, and a trailing metadata length; it preserves version, nil collections, and future slots.")
	presetSource := source("KCES 1.34.4", "game/KCES 1.34.4/Assembly-CSharp/MaidPreset.cs", "MaidPreset.LoadPreset/Serialize", 31, 180, "MaidPreset stores thumbnail, maiddata, and optional meta files in a VirtualDirectory; maiddata contains compressed MaidPresetCore property, color, and body byte blocks.")
	systemSource := source("KCES 1.34.4", "game/KCES 1.34.4/Assembly-CSharp/ApplicationSystemDataManager.cs", "ApplicationSystemDataManager.Load/Save", 12, 58, "ApplicationSystemDataManager opens system.dat as a VirtualDirectory and accesses typed EditData files below the EditData directory.")
	editDataSource := implementationSource("KCES 1.34.4", "serialization/KCES/system_dat.go", "KCESEditDataKindForPath/DecodeKCESSystemData", 76, 207, "The serializer mirrors the game path dispatch for preset-panel names, palette colors, gradation points, movable panels, color-preset order lists, and color presets; unknown files remain opaque.")
	ctSource := source("KCES 1.34.4", "game/KCES 1.34.4/WfSystem.FileSystem/Catalog/AssetBundleCatalog.cs", "AssetBundleCatalog", 15, 337, "The .ct catalog stores versioned catalog metadata, resource-file names, extension lists, hash-indexed items, and extension-name list side files used for resource lookup.")
	ctUtilitySource := source("KCES 1.34.4", "game/KCES 1.34.4/WfSystem.FileSystem/Catalog/CatalogUtility.cs", "CatalogUtility.FromCatalog/ToCatalog", 128, 306, "CatalogUtility stores catalog and extension lists as MessagePack virtual files inside a VirtualDirectory and resolves resource locations from their hashes and indices.")
	field := fieldFrom(presetSource, vdSource)

	preset := guide(
		"KCES .preset VirtualDirectory guide",
		"The current KCES character preset container. A version-1000 VirtualDirectory holds thumbnail, maiddata, and optional meta virtual files; this wire format is not the legacy COM3D2_PRESET format.",
		"runtime_verified",
		"MaidPreset and VirtualDirectory were reviewed in KCES 1.34.4. The outer file contract is verified; propData, colorData, and bodyData remain opaque byte blocks unless their dedicated inner codecs are used.",
		[]Source{presetSource, vdSource},
		[]Field{
			field("/format", "Editing format marker", "The marker for the KCES VirtualDirectory preset envelope.", "It identifies the current KCES preset representation and distinguishes it from legacy COM3D2_PRESET.", "editing_metadata", "Keep kces-virtual-directory-preset.", "critical"),
			field("/containerVersion", "Preset container version", "The outer VirtualDirectory version, normally 1000.", "VirtualDirectory serializes the preset using this version and its corresponding indexed metadata layout.", "version_marker", "Preserve the source value; use 1000 for a new current KCES preset.", "critical"),
			field("/thumbnail", "Preset thumbnail", "PNG thumbnail bytes stored in the thumbnail virtual file.", "MaidPreset decodes these bytes for the preset browser preview; they do not directly change character state.", "binary_asset", "Keep valid PNG data or preserve the original thumbnail when editing character blocks.", "medium"),
			field("/maidData", "Maid preset core", "The compressed MessagePack maiddata object containing versioned property, color, and body byte blocks.", "MaidPreset hands these blocks to Maid's property, multi-color, and body deserializers when applying a preset.", "runtime_configuration", "Keep the core version and all three inner blocks from the same source preset generation.", "critical"),
			field("/meta", "Preset metadata", "Optional compressed MessagePack metadata, normally including the preset display name.", "MaidPreset loads meta for preset-browser information and saves it as a separate virtual file.", "runtime_metadata", "Preserve keys understood by the target browser and retain unknown metadata entries.", "medium"),
			serializationField("/extraFiles", "Additional preset files", "Virtual files outside thumbnail, maiddata, and meta.", "MaidPreset ignores unknown files while VirtualDirectory retains them.", "opaque_preserve", "Preserve names and bytes unless a future game build documents them.", "high"),
			serializationField("/containerVersionless", "Versionless container flag", "Whether the decoded root used the historical versionless VirtualDirectory layout.", "VirtualDirectory selects its root indexed-array interpretation from this state.", "serialization_metadata", "Preserve the decoded layout flag.", "high"),
			serializationField("/containerFilesOnly", "Files-only container flag", "Whether the root omitted directory metadata in favor of a files-only layout.", "VirtualDirectory uses the flag while rebuilding the root table.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/containerDirectoriesNil", "Nil directory collection flag", "Whether the root directory collection was explicitly MessagePack nil.", "The distinction is part of the VirtualDirectory wire shape.", "serialization_metadata", "Keep it synchronized with containerDirectories.", "high"),
			serializationField("/containerFilesNil", "Nil file collection flag", "Whether the root file collection was explicitly MessagePack nil.", "The distinction is part of the VirtualDirectory wire shape.", "serialization_metadata", "Keep it synchronized with the virtual files and required preset files.", "high"),
			serializationField("/containerFieldCount", "Container field count", "The stored root indexed-array width.", "It preserves compatibility with future VirtualDirectory fields.", "serialization_metadata", "Preserve it when editing an existing preset.", "high"),
			serializationField("/containerFutureSlots", "Container future slots", "Raw MessagePack values beyond the known VirtualDirectory keys.", "Current KCES does not interpret these slots.", "opaque_preserve", "Preserve every slot.", "critical"),
			serializationField("/containerDirectories", "Directory metadata", "Per-directory version, nil, and future-slot metadata.", "VirtualDirectory uses it to recreate empty directories and unknown layout state.", "serialization_metadata", "Preserve keys and metadata exactly.", "high"),
			serializationField("/containerVirtualFiles", "Virtual-file metadata", "Per-file indexed metadata retained independently from file bytes.", "VirtualDirectory uses it to preserve short arrays and future slots for individual files.", "serialization_metadata", "Preserve keys and metadata exactly.", "high"),
		},
	)
	preset.FieldPatterns = []FieldPattern{
		pattern("/maidData/{version,propData,colorData,bodyData}", "Maid preset core block", "The core version and three opaque serialized Maid blocks.", "MaidPreset applies property, color, and body data through separate Maid deserializers.", "runtime_configuration", "Use the dedicated inner preset codec for a block edit; never swap a property block into a color or body slot.", presetSource),
		pattern("/meta/{version,metaData}", "Preset metadata block", "The metadata version and string map stored in the optional meta virtual file.", "MaidPreset exposes metadata such as presetName to the preset browser.", "runtime_metadata", "Keep the metadata version valid and preserve unknown keys.", presetSource),
		pattern("/container{Directories,VirtualFiles}/*", "VirtualDirectory child metadata", "Metadata for directories and files that are not represented by the typed fields above.", "VirtualDirectory uses this data for lossless reconstruction.", "serialization_metadata", "Do not rename keys or collapse nil and empty values.", vdSource),
	}
	preset.Rules = []Rule{{ID: "preset-inner-block-separation", AppliesTo: []string{"/maidData/propData", "/maidData/colorData", "/maidData/bodyData"}, Severity: "error", Summary: "The three Maid blocks have independent schemas.", Details: "MaidPreset passes propData, colorData, and bodyData to different deserializers. Decode and edit each block with its matching inner format; do not treat them as interchangeable JSON or raw MessagePack objects.", Evidence: []Source{presetSource}}}
	preset.Invariants = []string{"The native container contains thumbnail and maiddata; meta is optional.", "The current outer VirtualDirectory version is 1000.", "The current KCES preset wire is distinct from legacy COM3D2_PRESET despite sharing the .preset extension."}

	makeContainerFields := func(formatName string, includeCatalog bool) []Field {
		fields := []Field{
			field("/format", "Editing format marker", "The marker for the normalized VirtualDirectory/content-table envelope.", "It identifies the JSON representation and is not a native VirtualDirectory field.", "editing_metadata", "Keep "+formatName+".", "low"),
			field("/version", "Container version", "The VirtualDirectory version stored in the root indexed object.", "VirtualDirectory uses it to select the root layout and serialization callbacks.", "version_marker", "Preserve the decoded value; current KCES containers normally use 1000.", "high"),
			serializationField("/versionless", "Versionless root flag", "Whether the root used the historical versionless layout.", "VirtualDirectory chooses a different root field interpretation when this flag is set.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/filesOnly", "Files-only root flag", "Whether the root contains files without a directory collection.", "VirtualDirectory uses this flag while rebuilding the root table.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/directoriesNil", "Nil directory collection", "Whether the root directory collection was explicitly nil.", "Nil-versus-empty state is part of the indexed wire shape.", "serialization_metadata", "Keep it synchronized with directories.", "high"),
			serializationField("/filesNil", "Nil file collection", "Whether the root file collection was explicitly nil.", "Nil-versus-empty state is part of the indexed wire shape.", "serialization_metadata", "Keep it synchronized with files or virtualFiles.", "high"),
			serializationField("/fieldCount", "Root field count", "The stored indexed-array width for the container root.", "It preserves fields introduced by later builds.", "serialization_metadata", "Preserve it when editing an existing container.", "high"),
			serializationField("/futureSlots", "Root future slots", "Raw MessagePack values beyond the known root fields.", "Current KCES does not interpret them.", "opaque_preserve", "Preserve every slot.", "critical"),
			serializationField("/directories", "Directory metadata", "Per-directory layout metadata retained by the lossless codec.", "VirtualDirectory uses it to preserve empty directories and child layout state.", "serialization_metadata", "Preserve keys and metadata exactly.", "high"),
			serializationField("/virtualFiles", "Virtual-file metadata", "Per-file indexed metadata retained independently from file bytes.", "VirtualDirectory uses it to preserve short arrays and future slots for individual files.", "serialization_metadata", "Preserve keys and metadata exactly.", "high"),
		}
		if includeCatalog {
			fields = append(fields,
				field("/catalog", "Asset catalog", "The typed catalog virtual file decoded from the container.", "CatalogUtility and AssetBundleCatalog use catalog metadata, hashes, and resource indices to resolve game assets.", "runtime_catalog", "Keep catalog names, hashes, resource indices, and extension lists mutually consistent.", "critical"),
				field("/extensionNameLists", "Extension name lists", "Typed per-extension resource-name/hash lists stored as companion virtual files.", "AssetBundleCatalog.GetFileNameListFromExtension uses these lists to enumerate resources by extension.", "runtime_catalog", "Preserve each extension key and keep names/hashes aligned with catalog items.", "critical"),
				field("/files", "Unrecognized virtual files", "Raw named virtual files not decoded as catalog or extension-name-list records.", "The catalog loader ignores unknown side files but the container preserves them.", "opaque_preserve", "Preserve names and base64 bytes unless a source-reviewed file type is selected.", "high"),
			)
		}
		return fields
	}

	field = fieldFrom(ctSource, ctUtilitySource, vdSource)
	ct := guide(
		"KCES .ct catalog guide",
		"A VirtualDirectory content-table container whose catalog and extension-name lists drive AssetBundle resource lookup.",
		"runtime_verified",
		"AssetBundleCatalog, CatalogUtility, and the content-table reader were reviewed in KCES 1.34.4. Catalog hashes and index relationships are runtime-sensitive; unknown side files remain opaque.",
		[]Source{ctSource, ctUtilitySource, vdSource},
		makeContainerFields("kces-content-table", true),
	)
	ct.FieldPatterns = []FieldPattern{
		pattern("/catalog/{version,catalogType,packageType,priority,name,subName,hash,createTime,isEncrypted}", "Catalog identity and policy", "Version, catalog/category flags, package type, priority, names, hash, creation time, and encryption marker.", "NativeFileManager and CatalogUtility use these values to order catalogs and identify the corresponding .aba or archive.", "runtime_catalog", "Preserve catalog identity and recompute hashes only with the game's ignore-case hash function.", ctSource),
		pattern("/catalog/{resourceFileNames,extensionList,items,virtualItems}", "Catalog lookup tables", "Resource-file names, extension groups, and hash-indexed catalog items.", "AssetBundleCatalog maps an item resourceIndex to a resource file and locates entries by hash.", "runtime_catalog", "Keep resourceIndex within resourceFileNames and keep item hashes equal to the names used by the target catalog.", ctSource),
		pattern("/extensionNameLists/*/{extention,data}", "Extension resource index", "An extension string and its name/hash packs.", "GetFileNameListFromExtension returns this list for extension-based discovery.", "runtime_catalog", "Keep the game's misspelled JSON key extention and preserve name/hash pairs.", ctSource),
		pattern("/extensionNameLists/*/data/*/{name,hash}", "Extension name/hash pack", "One resource name and its case-insensitive FNV hash.", "Catalog lookup uses the hash for fast resolution and the name for the returned resource.", "runtime_catalog", "Recompute hash values whenever a name changes.", ctSource),
	}
	ct.Rules = []Rule{{ID: "catalog-reference-coherence", AppliesTo: []string{"/catalog", "/extensionNameLists"}, Severity: "error", Summary: "Catalog hashes and indices must remain coherent.", Details: "A catalog item points to resourceFileNames by resourceIndex and is found by its hash; extension-name lists repeat the same name/hash relationship. Update all copies together.", Evidence: []Source{ctSource, ctUtilitySource}}}
	ct.Invariants = []string{"catalog is required for a valid .ct envelope.", "resourceIndex values point into resourceFileNames.", "Hash fields use the game's case-insensitive resource-name hash convention."}

	field = fieldFrom(vdSource, ctUtilitySource)
	virtualDirectory := guide(
		"KCES VirtualDirectory guide",
		"The generic VirtualDirectory editing envelope used by KCES .vd containers that are not specialized as a bridge session, preset, system.dat, or .ct catalog.",
		"runtime_verified",
		"The VirtualDirectory signature, serialize-type dispatch, raw-file staging, and compressed metadata layout were reviewed in KCES 1.34.4. Catalog fields are optional and are only meaningful when this container actually stores a catalog.",
		[]Source{vdSource},
		makeContainerFields("kces-virtual-directory", true),
	)
	virtualDirectory.FieldPatterns = []FieldPattern{
		pattern("/files/*/{name,dataBase64}", "Virtual file payload", "A named raw virtual file and its base64 bytes.", "VirtualDirectory writes these bytes before serializing the directory metadata; consumers select files by path/name.", "runtime_resource_payload", "Preserve file names and bytes; use a specialized profile before editing a known file such as session_data or maiddata.", vdSource),
		pattern("/directories/*", "Virtual directory metadata", "Per-directory version, nil flags, and future slots.", "VirtualDirectory reconstructs the directory tree and preserves empty/future state from this metadata.", "serialization_metadata", "Preserve keys and metadata exactly.", vdSource),
		pattern("/virtualFiles/*", "Virtual file metadata", "Per-file indexed metadata not represented by the raw payload itself.", "VirtualDirectory uses it to reproduce short arrays and future fields for child file objects.", "serialization_metadata", "Preserve keys and metadata exactly.", vdSource),
		pattern("/catalog", "Optional catalog payload", "A typed AssetBundleCatalog when the generic container is used for catalog data.", "CatalogUtility uses the catalog to resolve resources; otherwise the field should remain null.", "runtime_catalog", "Only populate it when the corresponding virtual file is a catalog and keep its hash/index invariants valid.", ctUtilitySource),
	}
	virtualDirectory.Rules = []Rule{{ID: "specialize-known-virtual-files", AppliesTo: []string{"/files"}, Severity: "warning", Summary: "Known virtual files should use their specialized profile.", Details: "The generic envelope can preserve any file, but editing session_data, maiddata, system EditData, or catalog bytes without their specialized schema risks invalidating the consuming game subsystem.", Evidence: []Source{vdSource}}}

	field = fieldFrom(systemSource, vdSource, editDataSource)
	system := guide(
		"KCES system.dat guide",
		"The KCES user-system VirtualDirectory. Typed EditData files store editor palettes, gradation points, movable-panel state, color-preset order, and color presets; unknown files remain byte-preserved.",
		"runtime_verified",
		"ApplicationSystemDataManager, the VirtualDirectory implementation, and the KCES EditData path dispatch were reviewed in KCES 1.34.4.",
		[]Source{systemSource, editDataSource, vdSource},
		[]Field{
			field("/format", "Editing format marker", "The marker for the normalized system.dat envelope.", "It identifies the JSON representation and is not a native VirtualDirectory field.", "editing_metadata", "Keep kces-system-data.", "low"),
			field("/version", "System container version", "The outer VirtualDirectory version, normally 1000.", "ApplicationSystemDataManager loads and saves system.dat through VirtualDirectory using this version.", "version_marker", "Preserve the source value; use 1000 for a new current container.", "critical"),
			field("/editData", "Typed EditData files", "Recognized files under the EditData virtual directory, each carrying a path, kind discriminator, and one typed payload.", "ApplicationSystemDataManager and SceneEdit read these files to restore editor UI, palette, gradation, and color-preset state.", "runtime_configuration", "Keep path and kind synchronized and edit each union member with its matching schema.", "critical"),
			serializationField("/extraFiles", "Unknown system files", "Virtual files outside the recognized EditData path patterns.", "The game may use them in another subsystem or a future build; the current typed loader preserves them as raw bytes.", "opaque_preserve", "Preserve names and bytes; do not move a recognized path into extraFiles.", "critical"),
			serializationField("/versionless", "Versionless root flag", "Historical VirtualDirectory root-layout marker.", "It changes how the root indexed array is interpreted.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/filesOnly", "Files-only root flag", "Historical files-only root-layout marker.", "It changes how VirtualDirectory reconstructs the root.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/directoriesNil", "Nil directory collection", "Whether the root directory map was explicitly nil.", "Nil-versus-empty state is part of the wire layout.", "serialization_metadata", "Keep it synchronized with directories.", "high"),
			serializationField("/filesNil", "Nil file collection", "Whether the root file map was explicitly nil.", "Nil-versus-empty state is part of the wire layout.", "serialization_metadata", "Keep it synchronized with editData and extraFiles.", "high"),
			serializationField("/fieldCount", "Root field count", "Stored indexed-array width for the system container.", "It preserves fields introduced by future versions.", "serialization_metadata", "Preserve unchanged.", "high"),
			serializationField("/futureSlots", "Root future slots", "Raw MessagePack values beyond known VirtualDirectory keys.", "Current KCES does not interpret these slots.", "opaque_preserve", "Preserve every slot.", "critical"),
			serializationField("/directories", "Directory metadata", "Per-directory VirtualDirectory metadata.", "It preserves empty directories and child layout state.", "serialization_metadata", "Preserve keys and values exactly.", "high"),
			serializationField("/virtualFiles", "Virtual-file metadata", "Per-file VirtualDirectory metadata.", "It preserves child indexed-object layout state.", "serialization_metadata", "Preserve keys and values exactly.", "high"),
		},
	)
	system.FieldPatterns = []FieldPattern{
		pattern("/editData/*/{path,kind}", "EditData dispatch key", "The exact virtual path and inferred kind used to select a typed decoder.", "KCESEditDataKindForPath matches fixed names and prefixes such as EditData/PaletteColorSaveN and EditData/GradSvN.", "runtime_dispatch", "Do not rename or normalize paths; changing a path can change the selected union schema.", editDataSource),
		pattern("/editData/*/{presetPanelNames,paletteColor,gradPoints,moveablePanel,presetOrderList,colorPreset}", "Typed EditData union", "Exactly one payload corresponding to the entry kind.", "The system loader deserializes the selected MessagePack type and applies it to the editor subsystem.", "runtime_configuration", "Populate only the union member selected by kind and preserve indexed field counts/future slots inside it.", editDataSource),
		pattern("/editData/*/*/{fieldCount,futureSlots,rootNil,trailingData}", "EditData preservation metadata", "Indexed-object width, unknown slots, nil-root marker, and trailing bytes for a typed EditData payload.", "These values preserve compatibility and exact MessagePack shape.", "serialization_metadata", "Preserve unchanged unless rebuilding the matching inner object.", editDataSource),
	}
	system.Rules = []Rule{{ID: "editdata-path-kind", AppliesTo: []string{"/editData/*/path", "/editData/*/kind"}, Severity: "error", Summary: "EditData path determines the payload kind.", Details: "The game chooses a deserializer from the virtual path. A mismatched kind or moving a recognized file to extraFiles can make system.dat fail to load or silently ignore the edit.", Evidence: []Source{editDataSource}}}
	system.Invariants = []string{"system.dat is a VirtualDirectory, normally version 1000.", "Each recognized EditData path has exactly one matching union payload.", "Unknown virtual files are preserved rather than guessed."}

	return map[string]Guide{
		"kces.preset":           preset,
		"kces.ct":               ct,
		"kces.virtualdirectory": virtualDirectory,
		"kces.system":           system,
	}
}
