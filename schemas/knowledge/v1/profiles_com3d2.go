package knowledgev1

// com3d2Profiles 构建依据 COM3D2 与 COM3D2_5 游戏源码审核的格式指南 profile
// com3d2Profiles builds format-guide profiles reviewed against COM3D2 and COM3D2_5 game source
func com3d2Profiles() map[string]Guide {
	menuSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/Menu.cs", "Menu.ProcScriptBin", 156, 958, "The runtime reads the header and executes the command records in order; command state changes how later records are interpreted.")
	compileSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/ModCompile.cs", "ModCompile.CompileMenu", 45, 234, "The official compiler extracts name, category, and setumei metadata and writes the ordered command stream and header.")
	sceneSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/SceneEdit.cs", "SceneEdit.InitMenuItemScript", 1297, 1513, "The editor scans menu commands to build the visible name, description, category, icon, and item state.")
	menu3Source := source("COM3D2_5 3.48.0", "game/COM3D2_5 3.48.0/Assembly-CSharp/Menu.cs", "Menu.ProcScriptBin", 154, 1234, "The COM3D2_5 runtime retains the menu interpreter and adds CRC-body, split-item, mesh-morph, and body-state command branches.")
	compile3Source := source("COM3D2_5 3.48.0", "game/COM3D2_5 3.48.0/Assembly-CSharp/ModCompile.cs", "ModCompile.CompileMenuScript", 15, 228, "The COM3D2_5 text compiler validates additem, category, texture, and item-parameter forms before writing the same CM3D2_MENU stream.")
	scene3Source := source("COM3D2_5 3.48.0", "game/COM3D2_5 3.48.0/Assembly-CSharp/SceneEdit.cs", "SceneEdit.InitMenuItemScript", 1343, 1562, "The COM3D2_5 editor scan retains the metadata commands and priority/deletion/collaboration flags used by SceneEdit.")
	materialMgr3Source := source("COM3D2_5 3.48.0", "game/COM3D2_5 3.48.0/Assembly-CSharp/MaterialMgr.cs", "MaterialMgr.SetMatAlpha", 275, 285, "SetMatAlpha returns without effect for male skins or unloaded slot objects, then indexes the material array directly and applies the requested alpha.")
	field := fieldFrom(compileSource, menuSource, sceneSource, compile3Source, menu3Source, scene3Source, materialMgr3Source)

	menu := guide(
		"COM3D2 .menu editing guide",
		"A compiled COM3D2 item-menu script. The header contains compiler metadata and BodySize; Commands is an ordered program that changes item, model, material, texture, bone, and visibility state.",
		"runtime_verified",
		"The header, compiler, editor scan, and command interpreters were compared with COM3D2 2.48.0 and COM3D2_5 3.48.0. Command forms identify the build in which each syntax was observed; fields not listed here remain schema_only.",
		[]Source{compileSource, menuSource, sceneSource, compile3Source, menu3Source, scene3Source, materialMgr3Source},
		[]Field{
			field("/Signature", "Menu file signature", "The fixed BinaryWriter signature used to identify a compiled menu script.", "MenuHeader.Deserialize and the command loaders reject a header that is not CM3D2_MENU.", "format_marker", "Keep the exact value CM3D2_MENU.", "critical"),
			field("/Version", "Menu format version", "The compiler version marker stored after the signature.", "The reviewed compiler writes 1000. The current interpreter reads it for alignment but does not select a different command grammar from it.", "version_marker", "Preserve an existing value; use 1000 when creating a COM3D2 2.48.0 menu.", "high"),
			field("/SrcFileName", "Compiler source name", "The lower-case source path or file name copied into the header.", "The editor uses it for diagnostics; resource loading is driven by command arguments instead.", "diagnostic_metadata", "Keep an accurate source label, but do not treat it as a resource path.", "low"),
			field("/ItemName", "Header item-name snapshot", "A copy of the name command's display-name argument.", "The editor normally obtains the visible name from the command stream, so this header value can diverge from the effective UI name.", "duplicated_metadata", "Update it together with the corresponding name command.", "medium"),
			field("/Category", "Header category snapshot", "A copy of the category command's MPN category.", "The compiler validates the category and SceneEdit parses the command into an MPN; changing only this snapshot does not change the command result.", "duplicated_metadata", "Use an MPN present in the target build and update the category command as well.", "high"),
			field("/InfoText", "Header description snapshot", "A copy of the setumei command's description text.", "SceneEdit displays the command value and converts the game's line-break marker; the header is mainly metadata for readers.", "duplicated_metadata", "Update it together with setumei and preserve the game's line-break marker when needed.", "medium"),
			field("/BodySize", "Command-stream byte length", "The encoded byte length of the command region, including its terminal zero byte.", "The compiler calculates it from the encoded Commands. The interpreter consumes it as header data but terminates command parsing by argument counts and the zero terminator.", "derived", "Do not hand-edit it; let the writer recalculate it after Commands changes.", "high"),
			field("/Commands", "Ordered menu program", "The ordered list of command name and string-argument records.", "ProcScriptBin executes records from first to last, carrying category, slot, version, conditional, and temporary state into later records.", "runtime_program", "Make local edits only. Preserve order, duplicates, and the context established by preceding records.", "critical"),
		},
	)
	menu.FieldPatterns = []FieldPattern{
		pattern("/Commands/*/Command", "Command opcode", "The operation name written by the compiler.", "Menu.ProcScriptBin dispatches on this string.", "runtime_opcode", "Use a command whose argument contract is known; do not sort or normalize command names.", menuSource),
		pattern("/Commands/*/Args", "Command arguments", "Positional string arguments; their types are determined by the opcode and index.", "Branches parse them as MPNs, slots, numbers, colors, file names, bones, or conditions.", "runtime_arguments", "Edit arguments together with the opcode and preserve positional meaning.", menuSource),
	}
	menu.Commands = com3d2MenuCommands(compileSource, menuSource, sceneSource, compile3Source, menu3Source, scene3Source, materialMgr3Source)
	menu.ValueSets = com3d2MenuValueSets()
	menu.Sources = appendValueSetSources(menu.Sources, menu.ValueSets)
	menu.Rules = []Rule{
		{ID: "header-command-consistency", AppliesTo: []string{"/ItemName", "/Category", "/InfoText", "/Commands"}, Severity: "warning", Summary: "Header snapshots can duplicate command metadata.", Details: "The compiler copies name, category, and setumei into the header, while the editor and runtime primarily use Commands. Keep both representations consistent.", Evidence: []Source{compileSource, sceneSource}},
		{ID: "command-order", AppliesTo: []string{"/Commands"}, Severity: "error", Summary: "Command order is semantic.", Details: "Category, version, condition, slot, and temporary-state commands affect later records. Never sort or deduplicate the list.", Evidence: []Source{menuSource}},
		{ID: "command-form-target-build", AppliesTo: []string{"/Commands"}, Severity: "error", Summary: "Use a command form and value set reviewed for the target build.", Details: "Match the form's reviewed_in entry to the target game, preserve literal tokens shown in syntax, and resolve each argument value_set_refs entry for that build before selecting an MPN, slot, color channel, blend mode, or body-state value.", Evidence: []Source{menuSource, menu3Source}},
		{ID: "command-opcode-case", AppliesTo: []string{"/Commands/*/Command"}, Severity: "error", Summary: "Command opcode spelling is significant in compiled editing JSON.", Details: "The official text compiler lower-cases the first token, but direct binary serialization does not. Use the exact lower-case English or original Japanese opcode listed by command_semantics.", Evidence: []Source{compileSource, compile3Source, menuSource, menu3Source}},
		{ID: "derived-body-size", AppliesTo: []string{"/BodySize"}, Severity: "error", Summary: "BodySize is derived.", Details: "Recompute it from the encoded command stream rather than trying to preserve a stale number.", Evidence: []Source{compileSource}},
	}
	menu.Invariants = []string{"Signature is CM3D2_MENU.", "Commands are ordered and may contain duplicates.", "BodySize is derived from the encoded command region.", "A command's argument meaning depends on its opcode, position, form, and target-build value set."}

	materialSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/ImportCM.cs", "ImportCM.LoadMaterial/ReadMaterial", 317, 505, "The loader validates CM3D2_MATERIAL, reads the material name and shader, then applies typed texture, color, vector, float, range, offset, scale, and keyword properties to a Unity Material.")
	field = fieldFrom(materialSource)
	material := guide(
		"COM3D2 .mate material guide",
		"A CM3D2_MATERIAL material container. It identifies a material and applies a shader plus an ordered property stream to a Unity Material instance.",
		"runtime_verified",
		"ImportCM.LoadMaterial and ReadMaterial were reviewed in COM3D2 2.48.0; COM3D2_5 adds properties but retains the same container entry point.",
		[]Source{materialSource},
		[]Field{
			field("/Signature", "Material signature", "The fixed CM3D2_MATERIAL header string.", "ImportCM.LoadMaterial checks it before reading the material payload.", "format_marker", "Keep CM3D2_MATERIAL.", "critical"),
			field("/Version", "Material version", "The integer version stored after the material signature.", "The loader reads it to keep the stream aligned; property compatibility is selected by the property records and game build.", "version_marker", "Preserve the source value unless targeting a known compiler version.", "high"),
			field("/Name", "Material name", "The material instance name used by the loader and priority-material lookup.", "ImportCM assigns it to UnityEngine.Material.name and uses it when looking up a .pmat override.", "resource_identity", "Keep it synchronized with the material file and any matching priority-material entry.", "high"),
			field("/Material", "Material payload", "The shader identity and typed property records applied to the Unity material.", "ReadMaterial selects or clones a base material, changes the shader when needed, and applies each property in stream order.", "runtime_configuration", "Edit only known property variants; preserve property order and unknown records.", "critical"),
		},
	)
	material.FieldPatterns = []FieldPattern{
		pattern("/Material/{ShaderName,ShaderFilename}", "Shader identity", "The shader name and source label used by the material loader.", "A missing shader can leave the material unchanged or produce a warning.", "runtime_resource_reference", "Use a shader present in the target game build.", materialSource),
		pattern("/Material/Properties/*", "Typed material property", "A property record whose concrete type determines how its value is read and applied.", "The loader handles textures, colors, vectors, floats, ranges, offsets, scales, and keywords.", "runtime_property", "Keep the concrete property type and value representation together.", materialSource),
	}
	material.Rules = []Rule{{ID: "material-property-order", AppliesTo: []string{"/Material/Properties"}, Severity: "warning", Summary: "Preserve property order and concrete types.", Details: "The binary stream has no self-describing JSON type tag beyond each property's registered variant. Reordering or changing a variant can shift subsequent values.", Evidence: []Source{materialSource}}}
	material.Invariants = []string{"Signature is CM3D2_MATERIAL.", "Material must contain a payload before serialization.", "Shader and property resources must exist in the target build."}

	pmatSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/ImportCM.cs", "ImportCM.TryGetPriorityMaterial", 349, 390, "The loader reads CM3D2_PMATERIAL, stores the hash-keyed material name and render queue, and applies the queue when the material name matches.")
	field = fieldFrom(pmatSource)
	pmat := guide(
		"COM3D2 .pmat priority-material guide",
		"A priority-material override that changes a material's render queue and optionally records its shader name.",
		"runtime_verified",
		"The binary layout and the hash-keyed priority lookup were compared with ImportCM.TryGetPriorityMaterial in COM3D2 2.48.0.",
		[]Source{pmatSource},
		[]Field{
			field("/Signature", "Priority-material signature", "The CM3D2_PMATERIAL header string.", "ImportCM validates this string before adding the entry to the priority-material cache.", "format_marker", "Keep CM3D2_PMATERIAL.", "critical"),
			field("/Version", "Priority-material version", "The format version stored after the signature.", "The reviewed reader consumes it for layout compatibility; it does not use it as the render queue.", "version_marker", "Preserve the source value; use the target game's known version for new files.", "high"),
			field("/Hash", "Material lookup hash", "The integer key used to associate the override with a material name.", "ImportCM indexes the entry by this value and then checks that the resolved material name matches.", "derived_identity", "Keep it consistent with the game's hash function and MaterialName; let a format-aware writer recalculate it when supported.", "critical"),
			field("/MaterialName", "Target material name", "The material name whose render queue is overridden.", "ImportCM compares it to the loaded Unity Material.name before applying the override.", "resource_identity", "Use the exact material name, including case conventions used by the game.", "high"),
			field("/RenderQueue", "Render queue", "The Unity render queue value applied to the matching material.", "ImportCM sets _SetManualRenderQueue and Material.renderQueue from this value.", "runtime_numeric", "Change in small, intentional steps and verify sorting and transparency in-game.", "high"),
			field("/Shader", "Shader label", "The shader string stored in the priority-material record.", "It is part of the record and hash input in MeidoSerialization; the reviewed priority lookup primarily applies MaterialName and RenderQueue.", "resource_metadata", "Preserve it unless the target loader explicitly consumes it.", "medium"),
		},
	)
	pmat.Rules = []Rule{{ID: "priority-hash", AppliesTo: []string{"/Hash", "/MaterialName", "/Shader", "/RenderQueue"}, Severity: "error", Summary: "Keep the lookup hash coherent with the identity fields.", Details: "A mismatched hash can make an otherwise valid override unreachable or associate it with another material.", Evidence: []Source{pmatSource}}}
	pmat.Invariants = []string{"Signature is CM3D2_PMATERIAL.", "The lookup key must resolve to MaterialName.", "RenderQueue is a Unity queue value, not a material property name."}

	colSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/DynamicBone.cs", "DynamicBone.SerializeReadCollider", 232, 282, "The loader validates CM3D21_COL and constructs dbc, dpc, dbm, or missing collider objects, restoring their parent, transform, direction, center, and bound values.")
	field = fieldFrom(colSource)
	col := guide(
		"COM3D2 .col collider guide",
		"A CM3D21_COL list used by DynamicBone to create runtime collision components for hair, clothing, and other dynamic bones.",
		"runtime_verified",
		"DynamicBone.SerializeWriteCollider/SerializeReadCollider and all supported collider subclasses were reviewed in COM3D2 2.48.0.",
		[]Source{colSource},
		[]Field{
			field("/Signature", "Collider signature", "The CM3D21_COL header string.", "DynamicBone rejects another signature before reading collider records.", "format_marker", "Keep CM3D21_COL.", "critical"),
			field("/Version", "Collider version", "The integer version passed to each collider's serializer.", "The game changes the value between builds while retaining the reviewed record layout.", "version_marker", "Preserve the original value and use the target build's value for newly authored files.", "high"),
			field("/Colliders", "Collider records", "An ordered union list of DynamicBone collider records.", "Each record selects a concrete component and is attached to the named parent transform.", "runtime_collection", "Preserve order and use only supported type names; verify every parent bone exists.", "critical"),
		},
	)
	col.FieldPatterns = []FieldPattern{
		pattern("/Colliders/*/TypeName", "Collider type", "The concrete record tag: dbc, dpc, dbm, or missing.", "SerializeReadCollider selects DynamicBoneCollider, DynamicBonePlaneCollider, DynamicBoneMuneCollider, or an empty placeholder.", "union_discriminator", "Change the type together with its concrete fields.", colSource),
		pattern("/Colliders/*/{ParentName,SelfName,LocalPosition,LocalRotation,LocalScale}", "Collider transform", "The parent relationship and local transform restored on the generated GameObject.", "The component is found under the body root and its local transform is applied before simulation.", "runtime_transform", "Use existing bone names and valid finite vectors/quaternions.", colSource),
		pattern("/Colliders/*/{Direction,Center,Bound}", "Collider geometry", "Axis, center offset, and inside/outside constraint values for the concrete shape.", "DynamicBone collision resolution uses these values to constrain particles.", "runtime_geometry", "Keep Direction in 0..2 and Bound in the shape's supported range.", colSource),
	}

	phySource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/DynamicBone.cs", "DynamicBone.SerializeWrite/SerializeRead", 9, 230, "The physics writer and reader serialize root, partial-bone modes, base values, AnimationCurves, end behavior, gravity, force, collider references, exclusions, and freeze axis.")
	field = fieldFrom(phySource)
	phy := guide(
		"COM3D2 .phy DynamicBone guide",
		"The legacy DynamicBone simulation settings for one root bone chain. Scalar values can be global, curve-distributed, or partial per bone.",
		"runtime_verified",
		"DynamicBone.SerializeWrite/SerializeRead in COM3D2 2.48.0 was compared with the complete Phy layout. The file's version changes with builds but the reviewed field order is stable.",
		[]Source{phySource},
		[]Field{
			field("/Signature", "Physics signature", "The CM3D21_PHY header string.", "DynamicBone validates it before applying the settings.", "format_marker", "Keep CM3D21_PHY.", "critical"),
			field("/Version", "Physics version", "The build-specific integer passed to curve and collider readers.", "The game writes 24800 in the reviewed build; older files may contain another build number.", "version_marker", "Preserve an existing value; use the target game's value for new files.", "high"),
			field("/RootName", "Simulation root bone", "The transform name at which the dynamic particle chain starts.", "DynamicBone resolves this root before creating particles and applying the parameters.", "resource_identity", "Use a bone that exists on the target body and keep its case as stored by the game.", "critical"),
			field("/Damping", "Base damping", "The global damping scalar before optional curve or partial-bone overrides.", "The solver uses it to reduce particle motion; the distribution curve multiplies or varies it along the chain.", "runtime_numeric", "Keep values in the source game's expected range and edit the matching EnablePartial and curve fields together.", "high"),
			field("/Elasticity", "Base elasticity", "The global elasticity scalar for restoring particle positions.", "DynamicBone applies it during particle integration, optionally using a distribution curve.", "runtime_numeric", "Adjust with its distribution and partial mode as one group.", "high"),
			field("/Stiffness", "Base stiffness", "The global stiffness scalar controlling resistance to bending.", "The solver uses it to retain the initial bone-chain shape.", "runtime_numeric", "Change incrementally and test at multiple animation speeds.", "high"),
			field("/Inert", "Base inertia", "The global inertia scalar controlling how much particles follow parent motion.", "DynamicBone applies it during integration and can replace it with partial per-bone values.", "runtime_numeric", "Keep it coherent with EnablePartialInert and PartialInert.", "high"),
			field("/Radius", "Particle radius", "The base collision radius used by the particle solver.", "It expands the effective particle volume before collider correction.", "runtime_numeric", "Use non-negative values and validate against the target model scale.", "high"),
			field("/EndLength", "Virtual end length", "The length of a generated terminal particle when the chain has no child.", "A positive value creates a virtual end in the bone direction; EndOffset is the alternate explicit offset.", "topology_parameter", "Treat EndLength and EndOffset as an either/or terminal strategy.", "high"),
			field("/Gravity", "Gravity vector", "The gravity vector applied to the chain.", "DynamicBone adds it to particle integration after transforming it into the simulation context.", "runtime_vector", "Edit all three components together and verify stability at the target scale.", "high"),
			field("/Force", "External force vector", "A constant force added to particle integration.", "Unlike gravity, it is not used to preserve an initial gravity direction.", "runtime_vector", "Keep the magnitude modest; large values can cause tunneling or explosive motion.", "high"),
			field("/ColliderFileName", "Collider resource name", "The lower-case base name of the .col resource associated with this physics file.", "DynamicBone records it while loading the collider list and uses it as the current collider reference.", "resource_reference", "Keep it aligned with the deployed .col file name.", "high"),
			field("/FreezeAxis", "Freeze axis", "The axis constraint enum used when projecting particle motion.", "The solver supports None=0, X=1, Y=2, and Z=3 in the parent-bone local frame.", "runtime_enum", "Use only 0..3 and remember that the axes are local to each parent transform.", "high"),
		},
	)
	phy.FieldPatterns = []FieldPattern{
		pattern("/{DampingDistrib,ElasticityDistrib,StiffnessDistrib,InertDistrib,RadiusDistrib}", "Parameter distribution curve", "AnimationCurve keyframes containing time, value, and tangents.", "DynamicBone evaluates the curve along the chain to vary the corresponding base parameter.", "runtime_curve", "Keep times ordered and edit the base scalar and curve as one parameter.", phySource),
		pattern("/EnablePartial{Damping,Elasticity,Stiffness,Inert,Radius}", "Partial-mode selector", "The PartialMode enum selecting global/curve, per-bone, or legacy bone-name behavior.", "The reader switches between a scalar/curve and a per-bone dictionary based on this value.", "runtime_enum", "Use 0, 1, or 2 only and provide the matching partial list when mode 1 is selected.", phySource),
		pattern("/Partial{Damping,Elasticity,Stiffness,Inert,Radius}/*", "Per-bone parameter", "A bone name and value used when the corresponding partial mode is enabled.", "The game resolves the name against the dynamic particle list and stores the value per bone.", "runtime_collection", "Use exact runtime bone names; do not add entries for missing particles.", phySource),
	}
	phy.Rules = []Rule{{ID: "partial-mode-coherence", AppliesTo: []string{"/EnablePartialDamping", "/PartialDamping", "/DampingDistrib", "/EnablePartialElasticity", "/PartialElasticity", "/EnablePartialStiffness", "/PartialStiffness", "/EnablePartialInert", "/PartialInert", "/EnablePartialRadius", "/PartialRadius"}, Severity: "error", Summary: "Each partial selector must agree with its value representation.", Details: "Mode 0/curve uses the base scalar and distribution; mode 1 requires per-bone entries; mode 2 follows the legacy bone-name behavior. Keep the associated fields together.", Evidence: []Source{phySource}}}
	phy.Invariants = []string{"Signature is CM3D21_PHY.", "FreezeAxis is 0, 1, 2, or 3.", "A positive EndLength takes precedence over an explicit terminal EndOffset.", "Partial-mode selectors and their lists/curves must describe the same editing strategy."}

	pskSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/DynamicSkirtBone.cs", "DynamicSkirtBone.SerializeWrite/SerializeRead", 18, 140, "The skirt serializer writes CM3D21_PSK, radius and force curves, per-bone radius groups, stress and scale controls, velocity and gravity curves, and four hard values.")
	field = fieldFrom(pskSource)
	psk := guide(
		"COM3D2 .psk skirt-physics guide",
		"DynamicSkirtBone parameters for panier/skirt motion, including radius, force, stress response, gravity, and per-bone radius groups.",
		"runtime_verified",
		"DynamicSkirtBone.SerializeWrite/SerializeRead and its parameter update path were reviewed in COM3D2 2.48.0.",
		[]Source{pskSource},
		[]Field{
			field("/Signature", "Skirt-physics signature", "The CM3D21_PSK header string.", "DynamicSkirtBone rejects another signature before reading settings.", "format_marker", "Keep CM3D21_PSK.", "critical"),
			field("/Version", "Skirt-physics version", "The version controlling optional per-bone radius groups.", "Versions at or above the reviewed threshold contain the group records; older versions omit them.", "version_marker", "Preserve the source value and do not remove groups without checking the target build.", "high"),
			field("/PanierRadius", "Base panier radius", "The base radius used for skirt collision and deformation.", "DynamicSkirtBone combines it with its distribution curve and optional per-bone group values.", "runtime_numeric", "Change with the corresponding radius curves and verify skirt collisions.", "high"),
			field("/PanierForce", "Panier force", "The force driving skirt/panier motion.", "The solver applies it with its distribution and stress controls each update.", "runtime_numeric", "Use small changes and test both idle and animated poses.", "high"),
			field("/Gravity", "Skirt gravity vector", "The gravity vector used by the skirt solver.", "It is read as three floats and combined with GravityDistrib during parameter updates.", "runtime_vector", "Edit the vector and curve together; keep finite values.", "high"),
			field("/HardValues", "Hardness values", "The four hard/constraint coefficients written at the end of the file.", "DynamicSkirtBone uses them to shape the response of its native cloth plugin.", "runtime_numeric", "Preserve array length and change one coefficient at a time.", "high"),
		},
	)
	psk.FieldPatterns = []FieldPattern{
		pattern("/PanierRadiusDistribGroups/*", "Per-bone radius group", "A bone name, radius, and AnimationCurve override.", "The reader applies the group to the matching skirt bone when the version supports groups.", "runtime_collection", "Use exact bone names and preserve group order.", pskSource),
		pattern("/{PanierRadiusDistrib,PanierForceDistrib,VelocityForceRateDistrib,GravityDistrib}/*", "Skirt parameter keyframe", "A time/value/inTangent/outTangent keyframe.", "The solver evaluates these curves while updating skirt parameters.", "runtime_curve", "Keep keyframe times ordered and avoid extreme tangent overshoot.", pskSource),
	}

	anmSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/ImportCM.cs", "ImportCM.LoadAniClip", 770, 850, "The animation loader validates CM3D2_ANIM, maps property bytes to Unity AnimationCurve channels, and applies the curves to bone paths.")
	field = fieldFrom(anmSource)
	anm := guide(
		"COM3D2 .anm animation guide",
		"Legacy CM3D2_ANIM bone animation data. Each bone path contains property-indexed keyframe curves; version 1001 adds left/right bust animation switches.",
		"runtime_verified",
		"ImportCM.LoadAniClip and PhotoMotionData's version handling were reviewed in COM3D2 2.48.0; COM3D2_5 keeps the same property-channel model.",
		[]Source{anmSource},
		[]Field{
			field("/Signature", "Animation signature", "The CM3D2_ANIM header string.", "ImportCM validates it before constructing a Unity AnimationClip.", "format_marker", "Keep CM3D2_ANIM.", "critical"),
			field("/Version", "Animation version", "The version controlling optional bust-key flags.", "The reviewed reader accepts the channel stream and uses version-aware handling for BustKeyLeft and BustKeyRight.", "version_marker", "Preserve the original value; use 1001 when creating a modern file with bust flags.", "high"),
			field("/BoneCurves", "Bone curve records", "The ordered bone-path and property-curve records applied to an AnimationClip.", "The loader maps each property index to a Unity local position or rotation channel.", "runtime_animation", "Use valid bone paths and property indices; preserve keyframe order and channel identity.", "critical"),
			field("/BustKeyLeft", "Left bust animation switch", "Whether left-bust channels are enabled when supported by the file version.", "PhotoMotionData and the animation loader use this flag to include or suppress left-bust motion.", "runtime_toggle", "Change only when the corresponding channels are present.", "medium"),
			field("/BustKeyRight", "Right bust animation switch", "Whether right-bust channels are enabled when supported by the file version.", "The loader applies the same gating to right-bust channels.", "runtime_toggle", "Change only when the corresponding channels are present.", "medium"),
		},
	)
	anm.FieldPatterns = []FieldPattern{
		pattern("/BoneCurves/*/BonePath", "Animated bone path", "The Unity transform path receiving the curve channels.", "AnimationClip.SetCurve targets this path.", "runtime_resource_reference", "Use the exact path in the target model hierarchy.", anmSource),
		pattern("/BoneCurves/*/PropertyCurves/*/{PropertyIndex,Keyframes}", "Animation property curve", "A property index and Unity keyframes with time, value, and tangents.", "ImportCM maps the index to localPosition or localRotation and assigns an AnimationCurve.", "runtime_curve", "Use the documented property index range and finite keyframes.", anmSource),
	}

	modelSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/ImportCM.cs", "ImportCM.LoadSkinMesh_R", 46, 315, "The mesh loader validates CM3D2_MESH, builds the bone hierarchy and bind poses, fills vertices, normals, UVs, weights, submeshes, materials, and morph data, then creates a SkinnedMeshRenderer.")
	field = fieldFrom(modelSource)
	model := guide(
		"COM3D2 .model mesh guide",
		"A CM3D2_MESH skinned-model resource containing bones, bind poses, vertex channels, submeshes, material slots, and optional morph/thickness data.",
		"runtime_verified",
		"ImportCM.LoadSkinMesh_R was reviewed in COM3D2 2.48.0 and COM3D2_5 3.48.0. The loader's counts and conditional fields are part of the wire contract.",
		[]Source{modelSource},
		[]Field{
			field("/Signature", "Mesh signature", "The CM3D2_MESH header string.", "ImportCM refuses the stream when this signature is not present.", "format_marker", "Keep CM3D2_MESH.", "critical"),
			field("/Version", "Mesh version", "The version selecting optional scale and later mesh records.", "LoadSkinMesh_R uses it when reading bone transforms and model extensions.", "version_marker", "Preserve it and keep conditional fields consistent with the target build.", "critical"),
			field("/Name", "Model name", "The resource name used to name the generated GameObject.", "The loader prefixes it when creating the scene object and uses it as the mesh identity.", "resource_identity", "Keep it aligned with the deployed model resource.", "high"),
			field("/BoneNames", "Bone name table", "The ordered transform names used by bone weights and bind poses.", "ImportCM creates the hierarchy and resolves each weighted bone by this table.", "runtime_skeleton", "Do not reorder or rename entries without rewriting every dependent index and path.", "critical"),
			field("/Vertices", "Vertex positions", "The mesh vertex position array.", "The loader assigns it directly to the Unity Mesh before skinning and morph processing.", "runtime_geometry", "Keep its length equal to all vertex-indexed channels and preserve coordinate units.", "critical"),
			field("/BoneWeights", "Skin weights", "Four bone indices and weights per vertex.", "The SkinnedMeshRenderer uses them with BoneNames and BindPoses to deform the mesh.", "runtime_geometry", "Keep indices in range and weights coherent with the target bone count.", "critical"),
			field("/SubMeshes", "Submesh triangles", "The per-material triangle index arrays.", "ImportCM assigns each array to a Unity submesh and then binds material records in order.", "runtime_geometry", "Preserve submesh count and valid vertex indices.", "critical"),
			field("/Materials", "Model material slots", "Material records embedded in the mesh resource.", "The loader creates or resolves Unity materials and assigns them to the renderer's submesh slots.", "runtime_resource_reference", "Keep material count aligned with SubMeshes and use valid .mate resources.", "high"),
		},
	)
	model.FieldPatterns = []FieldPattern{
		pattern("/Bones/*", "Bone transform record", "A bone name, parent index, local position, rotation, and optional scale.", "LoadSkinMesh_R builds the hierarchy and applies these transforms before assigning bind poses.", "runtime_skeleton", "Parent indices must form a valid hierarchy; preserve order.", modelSource),
		pattern("/MorphData/*", "Morph target data", "Vertex deltas and metadata for a named morph target.", "ImportCM forwards morph records to TMorph for facial/body deformation.", "runtime_morph", "Keep vertex indices and delta arrays aligned with the model.", modelSource),
	}

	presetSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/CharacterMgr.cs", "CharacterMgr.PresetSave/PresetLoad/PresetSet", 998, 1210, "The preset writer emits CM3D2_PRESET, a type marker, thumbnail, property list, color data, and optional body data; the loader restores those blocks and applies them to a maid.")
	field = fieldFrom(presetSource)
	preset := guide(
		"COM3D2 .preset character guide",
		"A legacy CM3D2_PRESET character preset. It combines a preset type, thumbnail, maid property list, multi-color data, and optional body data.",
		"runtime_verified",
		"CharacterMgr's save/load/set paths and Maid's property/color/body serializers were reviewed in COM3D2 2.48.0.",
		[]Source{presetSource},
		[]Field{
			field("/Signature", "Preset signature", "The CM3D2_PRESET header string.", "CharacterMgr.PresetLoad rejects another signature.", "format_marker", "Keep CM3D2_PRESET.", "critical"),
			field("/Version", "Preset version", "The version controlling the nested Maid property layouts.", "CharacterMgr passes it to the nested deserializers and uses it when deciding which blocks are present.", "version_marker", "Preserve it and do not mix blocks from another preset generation.", "critical"),
			field("/PresetType", "Preset scope", "The enum selecting wear, body, or all character data.", "PresetSet applies the corresponding property, color, and body blocks to the selected maid.", "runtime_enum", "Use the target game's values 0=wear, 1=body, 2=all and keep matching blocks populated.", "high"),
			field("/ThumbData", "Preset thumbnail", "PNG thumbnail bytes stored with the preset.", "The preset browser decodes them for its preview; they do not change character state.", "binary_asset", "Keep valid PNG bytes or preserve the original thumbnail when editing other fields.", "medium"),
			field("/PresetPropertyList", "Maid property list", "The nested CM3D2_MPROP_LIST block containing part and attachment properties.", "CharacterMgr and Maid deserialize these values and apply menu parts, attachments, material properties, and bone lengths.", "runtime_configuration", "Edit only known properties and preserve map/order metadata and unknown nested records.", "critical"),
			field("/MultiColor", "Part color data", "The nested CM3D2_MULTI_COL color records.", "Maid applies them to part color and gradation state when the preset scope includes colors.", "runtime_configuration", "Keep part ordering and use ranges accepted by the target editor.", "high"),
			field("/BodyProperty", "Body data", "The nested CM3D2_MAID_BODY block used for body-scope presets.", "Maid restores body-specific state when PresetType includes body data.", "runtime_configuration", "Do not add body data to a wear-only preset unless the target loader expects it.", "high"),
		},
	)
	preset.Rules = []Rule{{ID: "preset-scope", AppliesTo: []string{"/PresetType", "/PresetPropertyList", "/MultiColor", "/BodyProperty"}, Severity: "error", Summary: "PresetType and nested blocks must agree.", Details: "A wear-only, body-only, or all-data preset is applied through different CharacterMgr branches. Preserve the block presence and type combination from a real file.", Evidence: []Source{presetSource}}}
	preset.Invariants = []string{"Signature is CM3D2_PRESET.", "Thumbnail length must equal the encoded thumbnail bytes.", "PresetType determines which nested data is applied.", "Nested property signatures must remain CM3D2_MPROP_LIST, CM3D2_MULTI_COL, and CM3D2_MAID_BODY where present."}

	timelineSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/DanceMain.cs", "DanceMain.Load / timeline_data.bytes", 200, 275, "DanceMain loads timeline_data.bytes and uses track data together with the animation timeline runtime.")
	timelineBinarySource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/AMBinaryDataBaseObject.cs", "AMBinaryDataBaseObject.Deserialize", 35, 65, "Object tracks resolve slash-separated Unity object paths before animation data is applied.")
	field = fieldFrom(timelineSource, timelineBinarySource)
	timeline := guide(
		"COM3D2 timeline editing guide",
		"The binary timeline_data.bytes model used by the dance/photo timeline system. Tracks are typed translation, rotation, property, or event streams.",
		"runtime_verified",
		"DanceMain, AMBinaryDataBaseObject, and the timeline track readers in the COM3D2 2.48.0 source were reviewed. Track IDs and object paths are cross-record references.",
		[]Source{timelineSource, timelineBinarySource},
		[]Field{
			field("/TotalFrame", "Timeline frame count", "The total number of frames in the timeline.", "Dance playback uses it to bound track sampling and playback duration.", "runtime_timing", "Keep it non-negative and consistent with every track's frame count.", "high"),
			field("/FrameRate", "Timeline frame rate", "The frames-per-second value used for playback timing.", "The dance runtime converts frame indices to time using this value.", "runtime_timing", "Use a positive value appropriate for the target timeline and update dependent event frames deliberately.", "high"),
			field("/Tracks", "Typed timeline tracks", "The ordered translation, rotation, property, and event track list.", "Each track resolves an object path and applies its samples or invokes events while the timeline advances.", "runtime_program", "Preserve track IDs, type tags, object paths, and sample counts.", "critical"),
		},
	)
	timeline.FieldPatterns = []FieldPattern{
		pattern("/Tracks/*/ObjectTreePath", "Animated object path", "A slash-separated path to the Unity object receiving a track.", "AMBinaryDataBaseObject resolves this path before applying values.", "runtime_resource_reference", "Use a path present in the target scene; avoid changing separators.", timelineBinarySource),
		pattern("/Tracks/*/{TrackID,TotalFrame}", "Track identity and length", "The numeric track ID and number of samples/events in the track.", "DanceObjectData references these IDs and the runtime iterates the declared length.", "runtime_reference", "Keep IDs unique where the source format requires them and match TotalFrame to arrays.", timelineSource),
		pattern("/Tracks/*/MethodDataArray/*", "Event method record", "A frame, component name, method name, and optional typed parameters.", "The event track invokes the named component method at the specified frame.", "runtime_event", "Only reference components and methods present in the target scene.", timelineSource),
	}

	objectSource := source("COM3D2 2.48.0", "game/COM3D2 2.48.0/Assembly-CSharp/DanceObjectDataBinary.cs", "DanceObjectDataBinary.Load/Save", 6, 130, "The dance loader maps object names and resource paths to referenced track IDs and resolves them in the scene, creating or locating objects as necessary.")
	field = fieldFrom(objectSource)
	objectData := guide(
		"COM3D2 dance object-data guide",
		"Object-reference tables shared by maid_data.bytes, item_data.bytes, and event_data.bytes. Each entry associates a scene object with timeline track IDs.",
		"runtime_verified",
		"DanceObjectDataBinary.Load/Save and its object lookup path were reviewed in COM3D2 2.48.0.",
		[]Source{objectSource},
		[]Field{
			field("/Entries", "Dance object entries", "The ordered table of target maid, object, resource, hierarchy path, and referenced track IDs.", "DanceObjectDataBinary resolves each entry to a GameObject and attaches the listed timeline tracks.", "runtime_reference_table", "Keep names and paths valid for the target scene and preserve track-ID relationships.", "critical"),
		},
	)
	objectData.FieldPatterns = []FieldPattern{
		pattern("/Entries/*/{ObjectName,TopObjectName,TreePath}", "Scene object identity", "Names and hierarchy path used to locate or create the target GameObject.", "The loader searches by name/path and logs an error when the object cannot be found.", "runtime_resource_reference", "Use exact scene names and slash-separated hierarchy paths.", objectSource),
		pattern("/Entries/*/ObjectReferenceTrackIDList/*", "Referenced track ID", "A timeline track ID applied to the entry's resolved object.", "Dance playback uses the association to route track samples to this object.", "runtime_reference", "Only reference tracks that exist in the companion timeline.", objectSource),
	}

	return map[string]Guide{
		"com3d2.menu":        menu,
		"com3d2.mate":        material,
		"com3d2.pmat":        pmat,
		"com3d2.col":         col,
		"com3d2.phy":         phy,
		"com3d2.psk":         psk,
		"com3d2.anm":         anm,
		"com3d2.model":       model,
		"com3d2.preset":      preset,
		"com3d2.timeline":    timeline,
		"com3d2.object_data": objectData,
	}
}
