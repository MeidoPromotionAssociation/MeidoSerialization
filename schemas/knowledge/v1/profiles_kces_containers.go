package knowledgev1

// kcesContainerProfiles 构建 KCES VirtualDirectory 格式和系统数据容器的源码审核指南
// kcesContainerProfiles builds source-reviewed guides for KCES VirtualDirectory formats and the system-data container
func kcesContainerProfiles() map[string]Guide {
	vdSource := source("KCES 1.34.4", "KCES 1.34.4/WfSystem.Serialization/VirtualDirectory.cs", "VirtualDirectory.Serialize/Deserialize", 409, 575, "VirtualDirectory writes a fixed signature, a serialize-type byte, raw virtual-file data, compressed indexed metadata, and a trailing metadata length using its declared current layout.")
	presetSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/MaidPreset.cs", "MaidPreset.LoadPreset/Serialize", 31, 180, "MaidPreset stores thumbnail, maiddata, and optional meta files in a VirtualDirectory; maiddata contains compressed MaidPresetCore property, color, and body byte blocks.")
	systemSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/ApplicationSystemDataManager.cs", "ApplicationSystemDataManager.Load/Save", 12, 58, "ApplicationSystemDataManager opens system.dat as a VirtualDirectory and accesses typed EditData files below the EditData directory.")
	editDataSource := implementationSource("KCES 1.34.4", "serialization/KCES/system_dat.go", "KCESEditDataKindForPath/DecodeKCESSystemData", 76, 207, "The serializer mirrors the game path dispatch for preset-panel names, palette colors, gradation points, movable panels, color-preset order lists, and color presets; only files outside every recognized path remain independent byte payloads.")
	ctSource := source("KCES 1.34.4", "KCES 1.34.4/WfSystem.FileSystem/Catalog/AssetBundleCatalog.cs", "AssetBundleCatalog", 15, 337, "The .ct catalog stores versioned catalog metadata, resource-file names, extension lists, hash-indexed items, and extension-name list side files used for resource lookup.")
	ctUtilitySource := source("KCES 1.34.4", "KCES 1.34.4/WfSystem.FileSystem/Catalog/CatalogUtility.cs", "CatalogUtility.FromCatalog/ToCatalog", 128, 306, "CatalogUtility stores catalog and extension lists as MessagePack virtual files inside a VirtualDirectory and resolves resource locations from their hashes and indices.")
	field := fieldFrom(presetSource, vdSource)

	preset := guide(
		"KCES .preset VirtualDirectory guide",
		"The current KCES character preset container. A version-1000 VirtualDirectory holds thumbnail, maiddata, and optional meta virtual files; this wire format is not the legacy COM3D2_PRESET format.",
		FormatVerificationSerializationVerified,
		"MaidPreset and VirtualDirectory were reviewed in KCES 1.34.4. The outer file contract is verified and propData, colorData, and bodyData use their dedicated typed inner codecs.",
		[]Source{presetSource, vdSource},
		[]Field{
			field("/format", "Editing format marker", "The marker for the KCES VirtualDirectory preset envelope.", "It identifies the current KCES preset representation and distinguishes it from legacy COM3D2_PRESET.", "editing_metadata", "Keep kces-virtual-directory-preset.", "critical"),
			field("/containerVersion", "Preset container version", "The outer VirtualDirectory version, normally 1000.", "VirtualDirectory serializes the preset using this version and its corresponding indexed metadata layout.", "version_marker", "Preserve the source value; use 1000 for a new current KCES preset.", "critical"),
			field("/thumbnail", "Preset thumbnail", "PNG thumbnail bytes stored in the thumbnail virtual file.", "MaidPreset decodes these bytes for the preset browser preview; they do not directly change character state.", "binary_asset", "Keep valid PNG data or preserve the original thumbnail when editing character blocks.", "medium"),
			field("/maidData", "Maid preset core", "The typed maiddata object containing versioned property, color, and body structures.", "MaidPreset hands the corresponding encoded blocks to Maid's property, multi-color, and body deserializers when applying a preset.", "runtime_configuration", "Keep the core version and all three typed structures from the same source preset generation.", "critical"),
			field("/meta", "Preset metadata", "Optional compressed MessagePack metadata, normally including the preset display name.", "MaidPreset loads meta for preset-browser information and saves it as a separate virtual file.", "runtime_metadata", "Preserve keys understood by the target browser and retain unknown metadata entries.", "medium"),
			serializationField("/extraFiles", "Additional preset files", "Independent virtual files whose names are outside thumbnail, maiddata, and meta.", "MaidPreset ignores these independently named files while VirtualDirectory stores their real byte-array payloads.", "binary_virtual_file", "Retain names and bytes only for files outside all three known names; known preset files must use typed fields and may never fall back here.", "high"),
			field("/containerDirectories", "Preset directories", "The actual child-directory version map in the VirtualDirectory container.", "VirtualDirectory recreates named directories before writing the preset's known and extra files.", "runtime_container", "Keep directory names and declared versions coherent with the preset file paths.", "high"),
		},
	)
	preset.FieldPatterns = []FieldPattern{
		pattern("/maidData/{version,propData,colorData,bodyData}", "Maid preset core block", "The core version and three typed Maid structures.", "MaidPreset applies property, color, and body data through separate Maid deserializers.", "runtime_configuration", "Use the matching typed structure for each block and never swap property, color, or body data.", presetSource),
		pattern("/meta/{version,metaData}", "Preset metadata block", "The metadata version and string map stored in the optional meta virtual file.", "MaidPreset exposes metadata such as presetName to the preset browser.", "runtime_metadata", "Keep the metadata version valid and preserve unknown keys.", presetSource),
		pattern("/containerDirectories/*/version", "Preset child-directory version", "The actual serialized version of one child VirtualDirectory.", "VirtualDirectory writes this version with the corresponding child directory.", "runtime_container", "Keep it compatible with the current KCES VirtualDirectory layout.", vdSource),
	}
	preset.Rules = []Rule{{ID: "preset-inner-block-separation", AppliesTo: []string{"/maidData/propData", "/maidData/colorData", "/maidData/bodyData"}, Severity: "error", Summary: "The three Maid blocks have independent schemas.", Details: "MaidPreset passes propData, colorData, and bodyData to different deserializers. Decode and edit each block with its matching inner format; do not treat them as interchangeable JSON or raw MessagePack objects.", Evidence: []Source{presetSource}}}
	preset.Invariants = []string{"The native container contains thumbnail and maiddata; meta is optional.", "The current outer VirtualDirectory version is 1000.", "The current KCES preset wire is distinct from legacy COM3D2_PRESET despite sharing the .preset extension."}

	makeContainerFields := func(formatName string, includeCatalog bool) []Field {
		fields := []Field{
			field("/format", "Editing format marker", "The marker for the normalized VirtualDirectory/content-table envelope.", "It identifies the JSON representation and is not a native VirtualDirectory field.", "editing_metadata", "Keep "+formatName+".", "low"),
			field("/version", "Container version", "The VirtualDirectory version stored in the root indexed object.", "VirtualDirectory uses it to select the root layout and serialization callbacks.", "version_marker", "Preserve the decoded value; current KCES containers normally use 1000.", "high"),
			field("/directories", "Directory metadata", "The actual per-directory version map in the container.", "VirtualDirectory recreates each named child directory from this map.", "runtime_container", "Preserve directory names and versions required by the contained files.", "high"),
		}
		if includeCatalog {
			fields = append(fields,
				field("/catalog", "Asset catalog", "The typed catalog virtual file decoded from the container.", "CatalogUtility and AssetBundleCatalog use catalog metadata, hashes, and resource indices to resolve game assets.", "runtime_catalog", "Keep catalog names, hashes, resource indices, and extension lists mutually consistent.", "critical"),
				field("/extensionNameLists", "Extension name lists", "Typed per-extension resource-name/hash lists stored as companion virtual files.", "AssetBundleCatalog.GetFileNameListFromExtension uses these lists to enumerate resources by extension.", "runtime_catalog", "Preserve each extension key and keep names/hashes aligned with catalog items.", "critical"),
				field("/files", "Unrecognized virtual files", "Independent named virtual files not declared as catalog or extension-name-list records.", "The catalog loader ignores these side files while the container stores their real byte-array payloads.", "binary_virtual_file", "Retain names and bytes only for files not declared by the catalog; a catalog or declared ExtensionNameList file must use its typed field and may never fall back here.", "high"),
			)
		}
		return fields
	}

	field = fieldFrom(ctSource, ctUtilitySource, vdSource)
	ct := guide(
		"KCES .ct catalog guide",
		"A VirtualDirectory content-table container whose catalog and extension-name lists drive AssetBundle resource lookup.",
		FormatVerificationSerializationVerified,
		"AssetBundleCatalog, CatalogUtility, and the content-table reader were reviewed in KCES 1.34.4. Catalog hashes and index relationships are runtime-sensitive; only independently named side files outside the catalog declarations remain byte payloads.",
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
		FormatVerificationSerializationVerified,
		"The VirtualDirectory signature, serialize-type dispatch, raw-file staging, and compressed metadata layout were reviewed in KCES 1.34.4. Catalog fields are optional and are only meaningful when this container actually stores a catalog.",
		[]Source{vdSource},
		makeContainerFields("kces-virtual-directory", true),
	)
	virtualDirectory.FieldPatterns = []FieldPattern{
		pattern("/files/*/{name,dataBase64}", "Virtual file payload", "A named virtual file whose actual native payload is a byte array.", "VirtualDirectory writes these bytes before serializing the directory metadata; consumers select files by path/name.", "runtime_resource_payload", "Use this form only for independent files without a specialized codec; known files such as session_data or maiddata must use their typed profile.", vdSource),
		pattern("/directories/*/version", "Virtual directory version", "The actual version value for one child directory.", "VirtualDirectory reconstructs the directory tree from these version records.", "runtime_container", "Preserve keys and versions exactly.", vdSource),
		pattern("/catalog", "Optional catalog payload", "A typed AssetBundleCatalog when the generic container is used for catalog data.", "CatalogUtility uses the catalog to resolve resources; otherwise the field should remain null.", "runtime_catalog", "Only populate it when the corresponding virtual file is a catalog and keep its hash/index invariants valid.", ctUtilitySource),
	}
	virtualDirectory.Rules = []Rule{{ID: "specialize-known-virtual-files", AppliesTo: []string{"/files"}, Severity: "error", Summary: "Known virtual files must use their specialized profile.", Details: "The generic envelope is only for independent files without a specialized codec. session_data, maiddata, recognized system EditData paths, catalog records, and declared ExtensionNameList files must be fully decoded and may never use dataBase64 as a parse-failure fallback.", Evidence: []Source{vdSource}}}

	field = fieldFrom(systemSource, vdSource, editDataSource)
	system := guide(
		"KCES system.dat guide",
		"The KCES user-system VirtualDirectory. Typed EditData files store editor palettes, gradation points, movable-panel state, color-preset order, and color presets; only independent files outside every recognized path remain byte payloads.",
		FormatVerificationSerializationVerified,
		"ApplicationSystemDataManager, the VirtualDirectory implementation, and the KCES EditData path dispatch were reviewed in KCES 1.34.4.",
		[]Source{systemSource, editDataSource, vdSource},
		[]Field{
			field("/format", "Editing format marker", "The marker for the normalized system.dat envelope.", "It identifies the JSON representation and is not a native VirtualDirectory field.", "editing_metadata", "Keep kces-system-data.", "low"),
			field("/version", "System container version", "The outer VirtualDirectory version, normally 1000.", "ApplicationSystemDataManager loads and saves system.dat through VirtualDirectory using this version.", "version_marker", "Preserve the source value; use 1000 for a new current container.", "critical"),
			field("/directories", "System directories", "The actual child-directory version map in system.dat.", "VirtualDirectory recreates the EditData and other child directories from these records.", "runtime_container", "Preserve directory names and versions required by typed and extra files.", "high"),
			field("/editData", "Typed EditData files", "Recognized files under the EditData virtual directory, each carrying a path, kind discriminator, and one typed payload.", "ApplicationSystemDataManager and SceneEdit read these files to restore editor UI, palette, gradation, and color-preset state.", "runtime_configuration", "Keep path and kind synchronized and edit each union member with its matching schema.", "critical"),
			serializationField("/extraFiles", "Independent system files", "Virtual files outside every recognized EditData path pattern.", "These files belong to another independently named subsystem while VirtualDirectory stores their real byte-array payloads.", "binary_virtual_file", "Retain names and bytes only for paths with no known decoder; a recognized path must use its typed union and may never move into extraFiles.", "critical"),
		},
	)
	system.FieldPatterns = []FieldPattern{
		pattern("/editData/*/{path,kind}", "EditData dispatch key", "The exact virtual path and inferred kind used to select a typed decoder.", "KCESEditDataKindForPath matches fixed names and prefixes such as EditData/PaletteColorSaveN and EditData/GradSvN.", "runtime_dispatch", "Do not rename or normalize paths; changing a path can change the selected union schema.", editDataSource),
		pattern("/editData/*/{presetPanelNames,paletteColor,gradPoints,moveablePanel,presetOrderList,colorPreset}", "Typed EditData union", "Exactly one payload corresponding to the entry kind.", "The system loader deserializes the selected MessagePack type and applies it to the editor subsystem.", "runtime_configuration", "Populate only the union member selected by kind; nullable roots use explicit JSON null.", editDataSource),
	}
	system.Rules = []Rule{{ID: "editdata-path-kind", AppliesTo: []string{"/editData/*/path", "/editData/*/kind"}, Severity: "error", Summary: "EditData path determines the payload kind.", Details: "The game chooses a deserializer from the virtual path. A mismatched kind or moving a recognized file to extraFiles can make system.dat fail to load or silently ignore the edit.", Evidence: []Source{editDataSource}}}
	system.Invariants = []string{"system.dat is a VirtualDirectory, normally version 1000.", "Each recognized EditData path has exactly one matching union payload.", "Only paths outside every recognized EditData pattern may appear in extraFiles."}

	return map[string]Guide{
		"kces.preset":           preset,
		"kces.ct":               ct,
		"kces.virtualdirectory": virtualDirectory,
		"kces.system":           system,
	}
}
