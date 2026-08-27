package knowledgev1

// KCES2 内衣「扒开／下拉」交互系统的两个配对资源指南
// .undressdat 是手工标注的设置数据，.undresspdat 是由它烘焙出来的预计算缓存
// Guides for the two paired resources of the KCES2 underwear peel-aside/pull-down interaction system
// .undressdat is the authored setup data and .undresspdat is the precomputed cache baked from it

const (
	kces2UndressUseModeValueSetID       = "kces2.undress.use_mode"
	kces2UndressFixModeValueSetID       = "kces2.undress.fix_mode"
	kces2UndressAutoSortModeValueSetID  = "kces2.undress.auto_sort_mode"
	kces2UndressSetupDataTypeValueSetID = "kces2.undress.setup_data_type"
	kces2UndressPeelCategoryValueSetID  = "kces2.undress.peel_category"
	kces2UndressRetensionValueSetID     = "kces2.undress.retension_category"
	kces2UndressFloatModeValueSetID     = "kces2.undress.float_mode"
)

// kcesUndressProfiles 构建 .undressdat 与 .undresspdat 的源码审核指南
// kcesUndressProfiles builds the source-reviewed guides for .undressdat and .undresspdat
func kcesUndressProfiles() map[string]Guide {
	return map[string]Guide{
		"kces.undressdat":  kcesUndressDataProfile(),
		"kces.undresspdat": kcesUndressPartsDataProfile(),
	}
}

// kcesUndressLoaderSource 引用同时读取两个文件并要求它们成对存在的加载入口
// kcesUndressLoaderSource references the loader entry point that reads both files and requires them to exist as a pair
func kcesUndressLoaderSource() Source {
	return source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/WearSetuper.cs", "WearSetuper.UpdateShortsSetup", 13, 22,
		"WearSetuper loads <garment>.undressdat and <garment>.undresspdat through GameResource.LoadBinary, aborts the whole undress setup when either is missing or empty, and parses them with JsonUtility.FromJson<ArchiveTarget> and JsonUtility.FromJson<PrecomputeTarget>.")
}

// kcesUndressDataProfile 构建 .undressdat 的源码审核指南
// kcesUndressDataProfile builds the source-reviewed guide for .undressdat
func kcesUndressDataProfile() Guide {
	loaderSource := kcesUndressLoaderSource()
	archiveSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ArchiveTarget.cs", "ArchiveTarget serialized members", 635, 674,
		"ArchiveTarget declares exactly fourteen [SerializeField] members, which are the fourteen members Unity JsonUtility reads and writes for a .undressdat file.")
	layerSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/OneLayer.cs", "OneLayer", 5, 43,
		"OneLayer stores the label, FixMode, AutoSortMode, and UseMode of one peel channel, and IsUniqueGroup and IsUnlock derive channel behavior from useMode and fixMode.")
	useModeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/UseModeUtil.cs", "UseModeUtil", 5, 194,
		"UseModeUtil classifies each UseMode as vertical or horizontal peeling, decides reverse direction, remaps the obsolete front values onto the crotch values, and GetString names them vertical, vertical reverse, front sideways, crotch, back sideways, and undress guide.")
	groupSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/OneGroup.cs", "OneGroup serialized members", 701, 779,
		"OneGroup stores one pullable ring of vertices: its label, behavior flags, guide-line labels, per-vertex weights and positions, owning layer, edge category, float mode, additional fixed vertex pairs, and the mesh vertex indices of the ring.")
	limitsSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ArchiveTarget.cs", "ArchiveTarget.PeelLimits", 802, 1056,
		"PeelLimits stores the per-step head and tail progress bounds, the per-group thresholds, the manual limit range, and the per-group horizontal-peel switches; FixHead and FixTail keep each head no greater than the tail of the same step, and CheckVer migrates format_version 0 to 1 to 2 while rewriting the thrs values in place.")
	limitsUseSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/HPeelPeelRailFilterCompo.cs", "HPeelPeelRailFilterCompo.ExecHPeel", 82, 100,
		"The horizontal peel rail filter resolves each thrs entry by group label and then overrides the trailing rail limits from the tails array.")
	selectSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/PeelDeflectionCompo.cs", "PeelDeflectionCompo.PickupValidList", 157, 172,
		"A group whose label appears in hPeelSelectLimits with value 0 is excluded from the valid horizontal-peel group list.")
	manualSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/PeelerExecParamDtoCompo.cs", "PeelerExecParamDtoCompo", 234, 242,
		"When manualLimitPac.valid is true the begin and end members replace the automatically derived peel limits.")
	vpeelSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ArchiveTarget.cs", "ArchiveTarget.VPeelExInfo", 1135, 1281,
		"VPeelExInfo property accessors fix the stored units: the three retension members are divided by 100 as percentages, vPeelFoldingWidth, vPeelFoldingCorrectWidth, vPeelVerticalFoldingWidthFront, and vPeelVerticalFoldingWidthBack are divided by 1000 as millimeters, and frontAdjustLength and backAdjustLength are used without conversion.")
	commonSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ArchiveTarget.cs", "ArchiveTarget.CommonPeelInfo", 744, 769,
		"CommonPeelInfo stores fixedPullLength, and the FixedPullLength accessor divides it by 1000, so the stored number is millimeters.")
	layerNumberSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ReadyPeelCompo.cs", "ReadyPeelCompo sub-mesh layer lookup", 218, 228,
		"Groups whose layer is 100 plus a sub-mesh ordinal are resolved as sub-mesh anchor groups, matching the MyEnum.LayerCategory range that starts at SUB_MESH_1 = 100, while ArchiveTarget.Validate pads the layers list to the MyConstants.MAXLayerCount of 15 channels.")
	formatterSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/ArchiveTargetFormatter.cs", "ArchiveTargetFormatter", 28, 133,
		"The constructor splits format on '.' and indexes three numeric components, and Exec runs the 1.0.1 through 1.2.2 migrations, which rewrite group labels, clone and delete layers, clear group indices, and finally set format to the migrated version.")

	archiveField := func(path, title, description, gameUsage, editRole, editGuidance, risk string, extra ...Source) Field {
		return field(path, title, description, gameUsage, editRole, editGuidance, risk, append([]Source{archiveSource}, extra...)...)
	}
	guideDoc := guide(
		"KCES2 .undressdat guide",
		"The authored setup data of the KCES2 underwear peel interaction, stored as a Unity JsonUtility document for UndressCore.ArchiveTarget.",
		FormatVerificationSerializationVerified,
		"The loader entry point, the complete set of serialized members, and the unit conventions of the numeric parameters were reviewed in KCES2 1.35.1. Every member is optional because JsonUtility leaves missing members at their defaults, and enum-typed members keep the stored integer because the game's own migrations retain obsolete values.",
		[]Source{loaderSource, archiveSource, formatterSource},
		[]Field{
			archiveField("/format", "Data format version", "The dotted version string of the setup data itself, such as 1.2.2.", "ArchiveTargetFormatter splits this string on '.' and indexes three numeric components, then runs every migration newer than the stored version and overwrites the member with the migrated version.", "format_version", "Keep a dotted string with at least three numeric components. Lowering it makes the game replay migrations that rewrite group labels and add or delete layers, and an empty or one-component value makes the version comparison read past the end of its array.", "critical", formatterSource),
			archiveField("/editVer", "Editor build version", "The version of the KCES2 undress editor that wrote the file.", "The runtime loader does not branch on this member; the paired .undresspdat carries its own copy of the same counter.", "provenance_metadata", "Preserve the source value and keep it equal to the editVer of the paired .undresspdat.", "medium"),
			archiveField("/meshRelPath", "Mesh relative path", "The relative path of the garment mesh this setup was authored against.", "ArchiveTargetFormatter.MakePeelMCategory searches this string for bra and pants to infer the peel category when setupDataType is None.", "resource_reference", "Keep it aligned with the real garment mesh; the bra and pants substrings affect category inference for files whose setupDataType is None.", "high", formatterSource),
			archiveField("/fbxName", "Target FBX name", "The FBX name of the garment this setup applies to.", "WearSetuper overwrites this member with the loaded garment resource name when setupDataType is None.", "resource_reference", "Keep it equal to the garment resource name that loads this file.", "high", loaderSource),
			archiveField("/setupDataType", "Setup data kind", "The eSeupDataType value selecting whether the file describes a body, pants, or bra setup.", "WearSetuper treats None as Pants and backfills fbxName, while ArchiveTargetFormatter maps this value onto the peel category during the 1.0.1 migration.", "runtime_enum", "Use a value from the reviewed eSeupDataType set and keep it consistent with peelCategory and the garment the file belongs to.", "critical", loaderSource, formatterSource),
			archiveField("/layers", "Peel channels", "The ordered list of peel channels, each declaring one peel direction plus its sort and lock behavior.", "The peel components select the groups of a channel by comparing each group's layer number against the channel position, and PickupAnyLayer looks a channel up by useMode. Validate pads this list to MyConstants.MAXLayerCount, that is 15 entries, while loading.", "runtime_collection", "Never reorder or delete entries without renumbering every dataGroup layer that names them; the position, not the label, selects the channel. Keep at least the 15 entries the game pads to so no channel silently shifts.", "critical", layerSource, layerNumberSource),
			archiveField("/subMeshRootVertexIndices", "Sub-mesh root vertices", "The root vertex index of each sub-mesh of the garment.", "The sub-mesh fixer resolves these vertices while setting up sliding sub-meshes.", "mesh_index_data", "Only use vertex indices that exist in the target mesh; these are precomputed mesh references rather than tunable values.", "critical"),
			archiveField("/subMeshRootVertexSubIndices", "Sub-mesh root sub-indices", "Additional slide vertex indices for each sub-mesh root vertex, where all-negative components mark an unused entry.", "SubMeshSlideSub.IsInvalid treats an entry with all three components negative as unset.", "mesh_index_data", "Keep this list aligned element by element with subMeshRootVertexIndices and use -1 for unused components.", "critical"),
			archiveField("/dataGroup", "Vertex groups", "The ordered list of pullable vertex groups; each group is one ring of mesh vertices plus the flags controlling how it deforms.", "The peel units read each group's indices as a pull rail, and the paired .undresspdat resolves its cached data back to these groups by matching layer and label.", "runtime_collection", "Treat the label and layer pair of every group as an identity shared with the paired .undresspdat; renaming, adding, deleting, or reordering groups invalidates that cache.", "critical", groupSource),
			archiveField("/tempIndex", "Group label counter", "The counter the editor uses when it allocates the next group label.", "MyConstants.getLayerGroupLabelName derives the next new label from this counter in the editor; the runtime peel path does not read it.", "editor_metadata", "Preserve the source value; lowering it makes the editor generate labels that collide with existing groups.", "low"),
			archiveField("/peelCategory", "Peel category", "The MyEnum.PeelCategory value classifying the garment as bra, shorts, other, or body.", "WearSetuper registers the garment in the character's wear list under this category, so it selects which slot the garment occupies.", "runtime_enum", "Use a value from the reviewed PeelCategory set and keep it consistent with setupDataType and meshRelPath.", "critical", loaderSource),
			archiveField("/hPeelLimits", "Horizontal peel limits", "The limit set for horizontal peeling: per-step progress bounds, per-group thresholds, a manual override range, and per-group participation switches.", "The rail filter resolves the per-group thresholds and the tail bounds, the deflection component drops groups switched off here, and the exec parameters honor the manual range when it is valid.", "runtime_parameters", "Keep each head no greater than the tail of the same step, and keep every label used here present in dataGroup.", "critical", limitsSource, limitsUseSource),
			archiveField("/vPeelExInfo", "Vertical peel parameters", "Nine numeric parameters controlling vertical peel adjustment, edge bite-in, and folding width.", "The vertical peel units read these numbers through accessors that divide the retension members by 100 and the folding members by 1000.", "runtime_parameters", "Respect the stored units: the retension members are percentages, the folding members are millimeters, and the two adjustLength members are already in Unity units.", "high", vpeelSource),
			archiveField("/commonPeelInfo", "Shared peel parameters", "The parameters shared by every peel mode.", "The peel units divide fixedPullLength by 1000 before use, so it is stored in millimeters.", "runtime_parameters", "Change fixedPullLength in millimeters and keep it positive.", "high", commonSource),
		},
	)

	guideDoc.FieldPatterns = []FieldPattern{
		pattern("/layers/*/label", "Peel channel label", "The editor-facing name of one peel channel, which may be empty.", "The runtime selects channels by useMode and by list position, not by this label.", "display_metadata", "Free text; changing it does not change peel behavior.", layerSource),
		pattern("/layers/*/{fixMode,autoSortMode,useMode}", "Peel channel behavior", "The FixMode lock state, the AutoSortMode direction used to order the vertices of a group into a pull rail, and the UseMode peel direction of the channel.", "UseModeUtil classifies the channel as vertical or horizontal peeling and derives its reverse flag, remapping the obsolete front values onto the crotch values; the sort mode fixes the direction along which a group's vertices form a rail.", "runtime_enum", "Use values from the reviewed FixMode, AutoSortMode, and UseMode sets. Do not introduce the obsolete UseMode values 20, 21, and 50 in new data: the 1.1.0 migration deletes the channels that use them.", layerSource, useModeSource),
		pattern("/dataGroup/*/{label,layer}", "Group identity", "The group label plus its layer number, which together identify the group. Layer numbers 0 through 14 name a peel channel, 100 and above name a MyEnum.LayerCategory sub-mesh layer, and the reviewed data also uses 15 through 17.", "OneGroupLooker keys its dictionary on this exact pair and RestoreDictionary resolves each cached key of the paired .undresspdat by matching both members. Labels repeat across layers in the reviewed data and the sub-mesh layer even uses an empty label, so neither member identifies a group on its own.", "resource_identity", "Keep every (layer, label) pair unique, and treat the pair as an identity shared with the paired .undresspdat. The label lists in hPeelLimits are resolved after the peel components have already narrowed the groups to one layer, so a label change must be mirrored there too.", groupSource, limitsSource, layerNumberSource),
		pattern("/dataGroup/*/{isInactive,isFloat,isOverPeel,isFloatGuide,isSolid,isPaste,isToForce,solidPriority}", "Group behavior flags", "The per-group switches selecting disabled, floating, over-peel, float-guide, solid, paste, and forced participation behavior, plus the solid-handling priority.", "The pre-peel and post-peel components branch on these flags to decide whether the group floats away from the body, is pasted onto its guide lines, is restored as a solid, or always participates.", "runtime_flags", "Change one flag at a time and verify in game: several flags only take effect together with the matching guide-line labels.", groupSource),
		pattern("/dataGroup/*/{floatGuideLine0,floatGuideLine1,pasteGuideLine0,pasteGuideLine1}", "Group guide-line references", "Labels of the other groups that act as float or paste guide lines for this group; an empty string means unset.", "The float and paste components resolve these labels to the guide groups they weakly reference while deforming this group.", "resource_reference", "Use labels that exist in dataGroup, or leave the empty string. A dangling label leaves the corresponding weak reference unresolved.", groupSource),
		pattern("/dataGroup/*/{vCategory,floatMode}", "Group edge and float classification", "The RetensionCategory edge classification of the group and the FloatMode value selecting which quadrant it floats toward.", "The retension components use the edge category to decide how the garment edge bites into the skin, and the float group component matches floatMode values 1 through 4 to pick the four floating groups.", "runtime_enum", "Use values from the reviewed RetensionCategory and FloatMode sets and keep left and right classifications consistent with the L and R suffixes of the group labels.", groupSource),
		pattern("/dataGroup/*/indices", "Group vertex rail", "The mesh vertex indices forming this group's pull rail, ordered by the channel's autoSortMode.", "The peel units walk this ordered index list as a rail and move the referenced vertices while the garment is pulled.", "mesh_index_data", "Only use vertex indices that exist in the target mesh. This is precomputed mesh data, not a tunable value; editing it by hand tears the mesh.", groupSource),
		pattern("/dataGroup/*/{weights,vertices,exFixeds}", "Group optional vertex data", "Optional per-vertex weights, baked vertex positions, and additional fixed vertex pairs of the group.", "All three are empty in the reviewed game data, so no runtime path exercising non-empty values was observed; exFixeds pairs a source vertex index with a destination vertex index.", "mesh_index_data", "Retain them exactly as stored, including empty arrays. Do not populate them from a mesh export without a game-side check.", groupSource),
		pattern("/hPeelLimits/{format_version,heads,tails}", "Peel limit bounds", "The version of the limit structure itself plus the per-step lower and upper progress bounds.", "The rail filter overrides the trailing rail limits from tails, FixHead and FixTail keep each head no greater than the tail of the same step, and CheckVer migrates format_version 0 to 1 to 2 while rewriting the thrs values in place.", "runtime_parameters", "Keep format_version at the stored value: lowering it makes the game rewrite every thrs threshold. Keep heads no greater than tails index by index, and do not shorten the arrays, which the game pads from static defaults.", limitsSource, limitsUseSource),
		pattern("/hPeelLimits/thrs/*/{label,thr}", "Per-group peel threshold", "One group label and the peel threshold applied to that group.", "The rail filter looks the label up among the peel rails and installs thr as that rail's limit.", "runtime_parameters", "Use labels present in dataGroup. Threshold values in the reviewed data range from -1 to 1 after the format_version 2 migration.", limitsSource, limitsUseSource),
		pattern("/hPeelLimits/hPeelSelectLimits/*/{label,value}", "Per-group peel switch", "One group label and the switch deciding whether the group participates in horizontal peeling.", "A group whose label appears here with value 0 is dropped from the valid horizontal-peel group list.", "runtime_flags", "Use labels present in dataGroup and only the reviewed values 0 and 1; an empty label marks the entry unset.", limitsSource, selectSource),
		pattern("/hPeelLimits/manualLimitPac/{valid,begin,end}", "Manual peel limit range", "A manual peel range that replaces the automatically derived limits when it is enabled.", "The exec parameter component substitutes begin and end for the derived limits whenever valid is true.", "runtime_parameters", "Only set valid when both bounds are authored deliberately; leaving stale bounds enabled overrides every automatic limit.", limitsSource, manualSource),
		pattern("/vPeelExInfo/*", "Vertical peel parameter", "One of the nine vertical peel adjustment, edge bite-in, and folding parameters.", "The accessors divide the three retension members by 100 and the four folding members by 1000, while the two adjustLength members are used unconverted.", "runtime_parameters", "Respect the unit of each member: percentages for retension, millimeters for folding, and Unity units for the two adjustLength members.", vpeelSource),
		pattern("/commonPeelInfo/fixedPullLength", "Fixed pull length", "The fixed pull length shared by every peel mode.", "The accessor divides it by 1000, so the stored number is millimeters.", "runtime_parameters", "Edit in millimeters and keep it positive.", commonSource),
	}

	guideDoc.ValueSets = kcesUndressValueSets()
	guideDoc.Rules = []Rule{
		{
			ID: "undress-pair-is-atomic", AppliesTo: []string{"/"}, Severity: "error",
			Summary:  "A .undressdat is only usable together with its .undresspdat of the same name.",
			Details:  "WearSetuper loads both files and aborts the whole undress setup when either one is missing or zero length, so the garment silently loses its peel behavior instead of degrading. Ship, edit, and version the two files as one unit.",
			Evidence: []Source{loaderSource},
		},
		{
			ID: "undress-group-identity-is-shared", AppliesTo: []string{"/dataGroup", "/dataGroup/*/label", "/dataGroup/*/layer", "/hPeelLimits/thrs", "/hPeelLimits/hPeelSelectLimits"}, Severity: "error",
			Summary:  "The label and layer pair of a group is an identity referenced from outside the file.",
			Details:  "The paired .undresspdat stores baked (layer, label) keys and refers to groups by their subscript in that baked list, while hPeelLimits keys its thresholds and switches on the label alone. Renaming, adding, deleting, or reordering groups without rebuilding the paired cache and both label lists makes the game log a missing-group error and drop that group's precomputed data.",
			Evidence: []Source{groupSource, limitsSource},
		},
		{
			ID: "undress-format-drives-migration", AppliesTo: []string{"/format", "/hPeelLimits/format_version"}, Severity: "error",
			Summary:  "Both version members trigger in-place data migration while loading.",
			Details:  "ArchiveTargetFormatter replays every migration newer than format, which rewrites group labels, clones and deletes layers, and clears group indices; PeelLimits.CheckVer independently migrates format_version and rewrites every thrs threshold. Lowering either member changes the data the game actually uses, and format must keep at least three dot-separated numeric components because the version comparison indexes three of them.",
			Evidence: []Source{formatterSource, limitsSource},
		},
	}
	guideDoc.Warnings = append(standardWarnings(), kcesUndressFloatTextWarning())
	return guideDoc
}

// kcesUndressPartsDataProfile 构建 .undresspdat 的源码审核指南
// kcesUndressPartsDataProfile builds the source-reviewed guide for .undresspdat
func kcesUndressPartsDataProfile() Guide {
	loaderSource := kcesUndressLoaderSource()
	precomputeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/PrecomputeTarget.cs", "PrecomputeTarget", 24, 126,
		"PrecomputeTarget declares exactly five [SerializeField] members, and InjectMeshReduction and InjectWidthMeasurer resolve each cached entry through OneGroupLooker before injecting it into the matching OneGroup, logging a missing-group error and skipping the entry when the lookup fails.")
	lookerSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/OneGroupLooker.cs", "OneGroupLooker", 12, 73,
		"BakeDictionary writes the group keys into Targets so that each key sits at its own index, GetValue resolves a cached index through Targets, and RestoreDictionary rebuilds the runtime dictionary by matching each key's lyr and lbl against the layer and label of the paired ArchiveTarget dataGroup.")
	degeneracySource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/VPeelTransPatchCompressor.cs", "VPeelTransPatchCompressor.DegeneracyTargetIndices", 136, 170,
		"DegeneracyTargetIndices.Exec moves every vertex listed in idcs onto the position of the idx vertex and returns immediately when idcs is null, which welds the mesh at that location.")
	widthSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/WidthMeasurerDto.cs", "WidthMeasurerDto", 5, 58,
		"WidthMeasurerDto captures hasValidWidthMeasurer plus the forward and reverse peel rail measurements of one group, and ReductionInfo stores the rail vertex ordinal, the normalized progress at that vertex, and the corresponding mesh vertex index.")

	precomputeField := func(path, title, description, gameUsage, editRole, editGuidance, risk string, extra ...Source) Field {
		return field(path, title, description, gameUsage, editRole, editGuidance, risk, append([]Source{precomputeSource}, extra...)...)
	}
	guideDoc := guide(
		"KCES2 .undresspdat guide",
		"The precomputed cache paired with a .undressdat of the same name, stored as a Unity JsonUtility document for UndressCore.PrecomputeTarget.",
		FormatVerificationSerializationVerified,
		"The loader entry point, the complete set of serialized members, and the way each cached entry is resolved back to a group of the paired .undressdat were reviewed in KCES2 1.35.1. Every value in this file is derived from the paired .undressdat rather than authored independently.",
		[]Source{loaderSource, precomputeSource},
		[]Field{
			precomputeField("/editVer", "Editor build version", "The version of the KCES2 undress editor that baked this cache.", "The runtime loader does not branch on this member; the paired .undressdat carries its own copy of the same counter.", "provenance_metadata", "Preserve the source value and keep it equal to the editVer of the paired .undressdat.", "medium"),
			precomputeField("/OneGroupLooker", "Group index table", "The baked table of group keys whose subscript is the index every other member of this file uses to name a group.", "RestoreDictionary matches each key's lyr and lbl against the layer and label of the paired .undressdat dataGroup, and GetValue resolves a cached index through this table.", "resource_identity", "Never reorder or delete entries: the subscript is the identity used by meshReduction and widthMeasurer. Rebuild the whole table from the paired .undressdat instead.", "critical", lookerSource),
			precomputeField("/WidthMeasurerValidPixelThreshold", "Width measurer pixel threshold", "The valid-pixel threshold the width measurer used while baking this cache.", "The width measurement pass compares against this threshold when deciding whether a measured rail is valid.", "runtime_parameters", "This is the only member of this file that is a tunable rather than a derived index; changing it without rebaking makes the cached measurements inconsistent with the threshold they were produced under.", "high"),
			precomputeField("/meshReduction", "Vertex degeneracy cache", "The per-group vertex degeneracy targets applied before and after peeling.", "InjectMeshReduction resolves each entry to its group and installs the two target sets, and the peel pass then moves every listed vertex onto its target vertex to weld the mesh.", "precomputed_cache", "Rebake it from the paired .undressdat rather than editing indices by hand; every index here refers to concrete mesh vertices.", "critical", degeneracySource),
			precomputeField("/widthMeasurer", "Width measurement cache", "The per-group forward and reverse peel rail measurements.", "InjectWidthMeasurer resolves each entry to its group and restores its valid flag and both rail measurements.", "precomputed_cache", "Rebake it from the paired .undressdat rather than editing measurements by hand.", "critical", widthSource),
		},
	)

	guideDoc.FieldPatterns = []FieldPattern{
		pattern("/OneGroupLooker/Targets/*/{lyr,lbl}", "Baked group key", "The peel channel subscript and group label locating one group of the paired .undressdat.", "RestoreDictionary matches this pair against the layer and label of a dataGroup entry and reports a missing group when nothing matches.", "resource_reference", "Keep every pair equal to a real (layer, label) pair of the paired .undressdat; a dangling key drops that group's cached data.", precomputeSource, lookerSource),
		pattern("/{meshReduction,widthMeasurer}/d/*/v/index", "Cached group reference", "The subscript into OneGroupLooker.Targets naming the group this cache entry belongs to.", "GetValue rejects an index beyond the end of Targets and otherwise resolves it to the group key and then to the group itself.", "resource_reference", "Keep every index within the bounds of OneGroupLooker.Targets and pointing at the intended group.", precomputeSource, lookerSource),
		pattern("/meshReduction/d/*/dat/{p,s}", "Degeneracy target set", "The vertex degeneracy target applied before or after peeling, holding the moved vertex indices and the vertex they are moved onto.", "Exec moves every vertex listed in idcs onto the position of the idx vertex and does nothing when idcs is empty, which welds the mesh at that location.", "precomputed_cache", "Only use vertex indices that exist in the target mesh, and retain empty index arrays exactly as stored.", degeneracySource),
		pattern("/widthMeasurer/d/*/dat/{hvp,info,infoR}", "Rail measurement", "The valid flag of the group plus the forward and reverse rail measurements, each holding a rail vertex ordinal, a normalized progress, and a mesh vertex index.", "Inject restores the valid flag and both rail measurements onto the group before the peel passes run.", "precomputed_cache", "Rebake rather than edit: the three numbers of a measurement are only consistent when produced together by the width measurer.", widthSource),
	}

	guideDoc.Rules = []Rule{
		{
			ID: "undress-pair-is-atomic", AppliesTo: []string{"/"}, Severity: "error",
			Summary:  "A .undresspdat is only usable together with its .undressdat of the same name.",
			Details:  "WearSetuper loads both files and aborts the whole undress setup when either one is missing or zero length, so the garment silently loses its peel behavior instead of degrading. Ship, edit, and version the two files as one unit.",
			Evidence: []Source{loaderSource},
		},
		{
			ID: "undress-precompute-is-derived", AppliesTo: []string{"/OneGroupLooker", "/meshReduction", "/widthMeasurer"}, Severity: "error",
			Summary:  "Every index in this file is derived from the paired .undressdat and must be rebaked, not edited.",
			Details:  "The cache addresses groups indirectly: a cache entry names a subscript into OneGroupLooker.Targets, and that entry names a (layer, label) pair of the paired dataGroup. Editing any of the three levels independently, or changing group labels or layers in the .undressdat, makes the lookup fail; the game then logs a missing-group error and silently drops that group's degeneracy and width data.",
			Evidence: []Source{precomputeSource, lookerSource},
		},
	}
	guideDoc.Warnings = append(standardWarnings(), kcesUndressFloatTextWarning())
	return guideDoc
}

// kcesUndressFloatTextWarning 说明本库写回原生文件时浮点文本与游戏侧写出的十进制展开不同
// kcesUndressFloatTextWarning explains that the float text this library writes back differs from the decimal expansion the game side writes
func kcesUndressFloatTextWarning() string {
	return "Native output is value-faithful but not byte-identical: Unity writes each float as a long decimal expansion such as 0.9900000095367432 while this library writes the shortest text that parses back to the same 32-bit float, such as 0.99. Every other byte of the document, including member order and indentation, is preserved."
}

// kcesUndressValueSets 构建 .undressdat 建模枚举成员的审核取值集合
// kcesUndressValueSets builds the reviewed value sets for the enum members modeled by .undressdat
func kcesUndressValueSets() []ValueSet {
	useModeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/UseMode.cs", "UseMode", 5, 22,
		"Declares the peel-direction values, including the three marked obsolete, with explicit non-sequential numbers.")
	useModeNameSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/UseModeUtil.cs", "UseModeUtil.GetString", 130, 192,
		"Names each UseMode for the editor as vertical, vertical reverse, front sideways right to left and left to right, crotch right to left and left to right, back sideways right to left and left to right, abolished, and undress guide 1.")
	fixModeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/FixMode.cs", "FixMode", 5, 11,
		"Declares the three sequential channel lock states, of which OneLayer.IsUnlock treats only Plain as unlocked.")
	autoSortSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/AutoSortMode.cs", "AutoSortMode", 5, 13,
		"Declares the five sequential directions used to order the vertices of a group into a pull rail.")
	setupTypeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/eSeupDataType.cs", "eSeupDataType", 5, 12,
		"Declares the four sequential setup-data kinds, and ArchiveTarget.IsBody and IsPants compare against Body and Pants.")
	peelCategorySource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/MyEnum.cs", "MyEnum.PeelCategory", 17, 24,
		"Declares the four garment categories plus the explicit sentinel value 99 for None.")
	retensionSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/RetensionCategory.cs", "RetensionCategory", 5, 14,
		"Declares the six sequential garment-edge categories, whose Japanese editor labels in RetensionCategoryStr read none, right front, right back, left front, left back, and belly.")
	floatModeSource := source("KCES2 1.35.1", "KCES2 1.35.1/Assembly-CSharp/UndressCore/FloatMode.cs", "FloatMode", 5, 14,
		"Declares the float quadrants plus a trailing COUNT sentinel, and FloatGroupsDtoComp selects the four floating groups by comparing floatMode against the unsigned values 1 through 4.")

	return []ValueSet{
		{
			ID:           kces2UndressUseModeValueSetID,
			CSharpType:   "UndressCore.UseMode",
			Description:  "Peel directions a layer can declare, with the non-sequential numbers stored in layers[].useMode.",
			EditGuidance: "Use 10 or 14 for vertical peeling and 30, 31, 40, or 41 for horizontal peeling. Do not author 20, 21, or 50: the 1.1.0 migration deletes the channels using them, and UseModeUtil remaps 20 and 21 onto 30 and 31.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values: []ValueSetValue{
				{Name: "None", Number: 0},
				{Name: "VPeel", Number: 10},
				{Name: "VPeelRev", Number: 14},
				{Name: "HPeelF1A", Number: 20},
				{Name: "HPeelF1B", Number: 21},
				{Name: "HPeelM1A", Number: 30},
				{Name: "HPeelM1B", Number: 31},
				{Name: "HPeelB1A", Number: 40},
				{Name: "HPeelB1B", Number: 41},
				{Name: "HPeelCommon_Unuse", Number: 50},
				{Name: "HWaistPress", Number: 90},
			},
			Evidence: []Source{useModeSource, useModeNameSource},
		},
		{
			ID:           kces2UndressFixModeValueSetID,
			CSharpType:   "UndressCore.FixMode",
			Description:  "Lock states of a peel channel stored in layers[].fixMode.",
			EditGuidance: "Only Plain leaves the channel unlocked; Pre and Fix restrict when the channel may move.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values:       sequentialValueSetValues([]string{"Plain", "Pre", "Fix"}),
			Evidence:     []Source{fixModeSource},
		},
		{
			ID:           kces2UndressAutoSortModeValueSetID,
			CSharpType:   "UndressCore.AutoSortMode",
			Description:  "Directions used to order the vertices of a group into a pull rail, stored in layers[].autoSortMode.",
			EditGuidance: "The sort direction fixes which end of a group is pulled first, so changing it reverses the peel direction of every group on that channel.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values:       sequentialValueSetValues([]string{"None", "XAsc", "XDesc", "YAsc", "YDesc"}),
			Evidence:     []Source{autoSortSource},
		},
		{
			ID:           kces2UndressSetupDataTypeValueSetID,
			CSharpType:   "UndressCore.eSeupDataType",
			Description:  "Setup-data kinds stored in the root setupDataType member.",
			EditGuidance: "None is not a neutral value: the loader rewrites it to Pants and backfills fbxName, and the 1.0.1 migration infers the peel category from meshRelPath instead.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values:       sequentialValueSetValues([]string{"None", "Body", "Pants", "Bra"}),
			Evidence:     []Source{setupTypeSource},
		},
		{
			ID:           kces2UndressPeelCategoryValueSetID,
			CSharpType:   "UndressCore.MyEnum.PeelCategory",
			Description:  "Garment categories stored in the root peelCategory member, including the explicit None sentinel.",
			EditGuidance: "The category selects the wear-list slot the garment occupies on the character, so keep it consistent with setupDataType and the garment itself.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values: []ValueSetValue{
				{Name: "Bra", Number: 0},
				{Name: "Shorts", Number: 1},
				{Name: "Other", Number: 2},
				{Name: "Body", Number: 3},
				{Name: "None", Number: 99},
			},
			Evidence: []Source{peelCategorySource},
		},
		{
			ID:           kces2UndressRetensionValueSetID,
			CSharpType:   "UndressCore.RetensionCategory",
			Description:  "Garment-edge categories stored in dataGroup[].vCategory.",
			EditGuidance: "Keep the left and right classification consistent with the L and R suffix of the group label; the edge components use it to decide how the garment edge bites into the skin.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values:       sequentialValueSetValues([]string{"None", "RightFront", "RightBack", "LeftFront", "LeftBack", "V5"}),
			Evidence:     []Source{retensionSource},
		},
		{
			ID:           kces2UndressFloatModeValueSetID,
			CSharpType:   "UndressCore.FloatMode",
			Description:  "Float quadrants stored as the unsigned dataGroup[].floatMode value, plus the trailing COUNT sentinel of the enum.",
			EditGuidance: "Only 1 through 4 select a floating group; 0 means the group does not float and COUNT is a sentinel rather than a usable value.",
			ReviewedIn:   []string{"KCES2 1.35.1"},
			Values:       sequentialValueSetValues([]string{"NONE", "FRONT_RIGHT", "FRONT_LEFT", "BACK_RIGHT", "BACK_LEFT", "COUNT"}),
			Evidence:     []Source{floatModeSource},
		},
	}
}
