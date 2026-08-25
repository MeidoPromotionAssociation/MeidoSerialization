package knowledgev1

// kcesPayloadProfiles 构建各原生后缀载荷语义的审核指南，编辑 JSON 根即载荷对象本身
// kcesPayloadProfiles builds reviewed guides for the native-suffix payload semantics whose editing JSON root is the payload object itself
func kcesPayloadProfiles() map[string]Guide {
	payloadSource := implementationSource("KCES 1.34.4", "serialization/KCES/payload.go", "DecodeKCESPayload/EncodeKCESPayload", 20, 330, "The dispatcher binds every supported extension to one typed payload root, fully decodes that root, and rejects unknown extensions or trailing MessagePack data.")
	magicaSource := implementationSource("KCES 1.34.4", "serialization/KCES/magica_cloth_serialize_data.go", "MagicaClothSerializeData", 20, 210, "Every ClothSerializeData member is an optional pointer so decoding keeps exactly the members the original file contained and encoding writes back only those members.")
	dynamicMgrSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/DynamicBoneMgr.cs", "DynamicBoneMgr.LoadDynamicYureBoneSetting/LoadMagica2DynamicYureBoneSetting", 212, 485, "DynamicBoneMgr selects .dbconf/.dbcol, .db2conf, .dsbconf/.dsb2conf, .dslconf/.dsl2conf/.dslcol, and default fallback names for the corresponding physics components.")
	dynamicSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/DynamicYureBone.cs", "DynamicYureBone.Save/Load/SaveCollider/LoadCollider", 281, 400, "DynamicYureBone serializes DynamicBoneStatus and DynamicBoneColliderData using MessagePack Lz4BlockArray and reads an Int32 compressed-byte length before deserializing.")
	sleeveSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/DynamicSleeveBone.cs", "DynamicSleeveBone.SaveParams/LoadParams/LoadCollider", 1080, 1180, "DynamicSleeveBone loads ClothParams from .dslconf, optional MagicaCloth2 JSON from .dsl2conf, and collider data from .dslcol.")
	skirtSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/DynamicKCESSkirtBone.cs", "DynamicKCESSkirtBone.SaveParams/LoadParams/LoadMagica2Params", 120, 150, "DynamicKCESSkirtBone uses ClothParams for .dsbconf and a JSON string for .dsb2conf.")
	ikSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/kt/ik/IKColliderSaveLoader.cs", "IKColliderSaveLoader.Save/Load", 66, 145, "IKColliderSaveLoader reads a length-prefixed Lz4BlockArray IKColliderDataPackage and creates collider objects for each effector group.")
	limbSource := source("KCES 1.34.4", "KCES 1.34.4/Assembly-CSharp/LimbColliderMgr.cs", "LimbColliderMgr.Save/Load", 108, 140, "LimbColliderMgr reads a length-prefixed Lz4BlockArray LimbColliderPackage and applies each target limb collider status to the generated limb collider.")
	colliderSource := implementationSource("KCES 1.34.4", "serialization/KCES/collider_payload.go", "ColliderPackage/ColliderRef and collider status unions", 17, 850, "The codec models the four game-defined plane, capsule, sphere, and maid-property collider tags and rejects every other union tag.")
	clothSource := implementationSource("KCES 1.34.4", "serialization/KCES/cloth_params.go", "ClothParams", 107, 230, "ClothParams mirrors MagicaCloth's indexed keys 0..82, including sparse holes at keys 4, 5, and 56, Bezier parameters, constraints, and mode enums.")

	dynamicFields := func() []FieldPattern {
		return []FieldPattern{
			pattern("/{version,damping,elasticity,stiffness,inert,radius}", "DynamicBone scalar status", "The indexed DynamicBoneStatus version and base damping, elasticity, stiffness, inertia, and radius values.", "DynamicYureBone applies these values to its DynamicBone solver before simulation.", "runtime_physics", "Edit a base value together with its optional keyframe curve and test the resulting motion at the target scale.", dynamicSource),
			pattern("/{endLength,endOffset,gravity,force,freezeAxis}", "DynamicBone terminal and force settings", "Virtual-end geometry, gravity, external force, and freeze-axis values.", "DynamicBone uses these values during particle integration and terminal-particle creation.", "runtime_physics", "Keep EndLength/EndOffset mutually coherent, use finite vectors, and use FreezeAxis values supported by the target build.", dynamicSource),
			pattern("/*KeyFrames/*/{time,value,inTangent,outTangent}", "DynamicBone keyframe", "A time/value/tangent record for one distributed DynamicBone parameter.", "The solver evaluates these curves along the particle chain.", "runtime_curve", "Keep keyframe times ordered and edit tangents conservatively to avoid unstable overshoot.", dynamicSource),
		}
	}

	colliderFields := func() []FieldPattern {
		return []FieldPattern{
			pattern("/{version,colliders,limbEnableList}", "Collider package root", "The indexed collider-package version, collider references, and optional limb enable list.", "DynamicYureBone applies collider statuses and limb toggles when loading .dbcol or a compatible sidecar.", "runtime_collision", "Keep the package version and list shapes coherent; preserve list order and nil-versus-empty state.", colliderSource),
			pattern("/colliders/*/{type,collider}", "Collider union entry", "A type tag paired with one of the four collider status structures defined by the game.", "The loader creates a concrete collider component from the tag and status object.", "runtime_collision", "Change type and concrete payload together; unsupported tags must be rejected rather than retained as raw data.", colliderSource),
			pattern("/colliders/*/collider/{version,parentName,selfName,localPosition,localRotation,localScale,center,bound}", "Collider transform and bounds", "Common collider status fields restored on the runtime collider object.", "The dynamic-bone solver resolves parent/self names and applies local transforms, center, and inside/outside bound mode.", "runtime_collision", "Use existing transform names, finite vectors/quaternions, and a supported bound value.", colliderSource),
			pattern("/colliders/*/collider/{direction,isDirectionInverse,startRadius,endRadius,height,radius}", "Collider shape parameters", "Axis, reversal, capsule dimensions, and sphere radius for concrete collider types.", "DynamicYureBone uses these fields to construct plane, capsule, sphere, or maid-property geometry.", "runtime_geometry", "Only edit fields present for the selected type; keep dimensions non-negative and aligned with model scale.", colliderSource),
			pattern("/colliders/*/collider/{centerMpnList,centerRateMax,startRadiusMpnList,maxStartRadius,endRadiusMpnList,maxEndRadius,centerMpnNameList,startRadiusMpnNameList,endRadiusMpnNameList}", "Maid-property collider scaling", "Body-property lists and maxima used by maid-property colliders to scale center and radii.", "The game recomputes these dimensions from maid MPN values when the body changes.", "runtime_body_dependent", "Keep enum lists and name lists paired and preserve values required by the target body type.", colliderSource),
			pattern("/limbEnableList/*/{version,limbType,isEnable}", "Limb collider enable state", "A limb type and enabled flag carried alongside dynamic-bone collider records.", "DynamicYureBone toggles generated limb colliders from this list.", "runtime_collision", "Use limb enum values present in the target build and preserve list ordering.", colliderSource),
		}
	}

	clothFields := func() []FieldPattern {
		return []FieldPattern{
			pattern("/{radius,mass,gravity,drag,maxVelocity,worldMoveInfluence,worldRotationInfluence,clampPositionLength,clampRotationAngle,structDistanceStiffness,bendDistanceStiffness,nearDistanceLength,nearDistanceStiffness,restoreRotation,triangleBend,volumeStretchStiffness,volumeShearStiffness,penetrationConnectDistance,penetrationDistance,penetrationRadius,springDirectionAtten,springDistanceAtten}", "Cloth Bezier parameter", "A BezierParam with start/end values and optional curve value switches.", "MagicaCloth evaluates these parameters along cloth particles when the corresponding constraint is enabled.", "runtime_cloth_parameter", "Keep use flags consistent with the supplied values and preserve the target cloth's scale and units.", clothSource),
			pattern("/{useGravity,useDrag,useMaxVelocity,useDistanceDisable,useResetTeleport,useClampDistanceRatio,useClampPositionLength,useClampRotation,useBendDistance,useNearDistance,useRestoreRotation,useSpring,useTriangleBend,useVolume,useCollision,usePenetration,useLineAvarageRotation,useFixedNonRotation}", "Cloth constraint switch", "Boolean switches enabling individual MagicaCloth forces and constraints.", "DynamicSleeveBone and DynamicKCESSkirtBone pass these switches to the cloth solver.", "runtime_cloth_constraint", "Edit a switch together with its parameter block; preserve the game's Avarage spelling in useLineAvarageRotation.", clothSource),
			pattern("/{massInfluence,windInfluence,windRandomScale,disableDistance,disableFadeDistance,teleportDistance,teleportRotation,clampDistanceMinRatio,clampDistanceMaxRatio,clampDistanceVelocityInfluence,clampPositionRatioX,clampPositionRatioY,clampPositionRatioZ,clampPositionVelocityInfluence,clampRotationVelocityInfluence,restoreDistanceVelocityInfluence,nearDistanceMaxDepth,restoreRotationVelocityInfluence,springPower,springRadius,springScaleX,springScaleY,springScaleZ,springIntensity,adjustRotationPower,maxVolumeLength,friction,penetrationMaxDepth,maxMoveSpeed,maxRotationSpeed,resetStabilizationTime,clampRotationVelocityLimit}", "Cloth scalar constraint value", "A numeric MagicaCloth force, distance, velocity, friction, or stabilization parameter.", "The cloth solver consumes these values when the related constraint or force is active.", "runtime_cloth_parameter", "Keep finite values in the target game's expected units and test both static and animated poses.", clothSource),
			pattern("/{adjustMode,penetrationMode,penetrationAxis,teleportMode,bendDistanceMaxCount,nearDistanceMaxCount}", "Cloth mode or count", "An enum or bounded iteration count selecting cloth adjustment, penetration, teleport, or neighbor-distance behavior.", "MagicaCloth branches on these values while updating particles and collision constraints.", "runtime_enum", "Use enum values from the target KCES build and keep counts non-negative.", clothSource),
		}
	}

	ikFields := func() []FieldPattern {
		return []FieldPattern{
			pattern("/{version,groups}", "IK collider package root", "The indexed package version and effector-group list.", "IKColliderSaveLoader iterates groups and creates collider objects for each full-body IK effector.", "runtime_ik_collision", "Preserve group order and use effector enum values from the target FullBodyIK build.", ikSource),
			pattern("/groups/*/{version,target,colliders}", "IK effector collider group", "An effector target and its ordered collider references.", "The loader names and attaches colliders to the matching IK effector.", "runtime_ik_collision", "Keep target values and collider lists aligned with the target rig; edit union types with their payloads.", ikSource),
			pattern("/groups/*/colliders/*/{type,collider}", "IK collider union entry", "A typed collider reference using one of the four game-defined union tags.", "IKColliderSaveLoader creates the corresponding native collider component from the status.", "runtime_collision", "Preserve type/payload agreement and reject unsupported union tags.", ikSource),
		}
	}

	limbFields := func() []FieldPattern {
		return []FieldPattern{
			pattern("/{version,items}", "Limb collider package root", "The indexed package version and limb-item list.", "LimbColliderMgr reads the list and applies each status to its generated limb collider.", "runtime_limb_collision", "Preserve item order and use limb targets present in the target build.", limbSource),
			pattern("/items/*/{version,target,collider}", "Limb collider item", "A limb enum target paired with the concrete NativeMaidPropColliderStatus structure used by LimbColliderMgr.", "LimbColliderMgr looks up the target limb and calls SetStatus with the collider data.", "runtime_limb_collision", "Keep target and collider geometry matched and reject any incompatible collider representation.", limbSource),
		}
	}

	clothRootFields := func(src Source) []Field {
		return []Field{
			field("/radius", "Cloth particle radius", "The particle-radius BezierParam evaluated along the cloth chain.", "MagicaCloth sizes every cloth particle from this parameter before resolving collisions.", "runtime_cloth_parameter", "Keep the start/end values and use flags coherent with the target model scale.", "critical", src, clothSource),
			field("/mass", "Cloth particle mass", "The particle-mass BezierParam evaluated along the cloth chain.", "MagicaCloth weights every cloth particle from this parameter during integration.", "runtime_cloth_parameter", "Keep the values positive and coherent with the other force parameters.", "high", src, clothSource),
			field("/gravityDirection", "Cloth gravity direction", "The world-space gravity direction vector.", "MagicaCloth applies gravity along this direction while useGravity is set.", "runtime_cloth_parameter", "Use a finite vector and edit it together with useGravity and the gravity parameter.", "high", src, clothSource),
		}
	}

	magicaRootFields := func(src Source) []Field {
		return []Field{
			field("/clothType", "MagicaCloth2 cloth type", "The ClothSerializeData cloth-type enum.", "MagicaCloth2 selects its simulation model from this value.", "runtime_json_configuration", "Change this only together with the members the selected cloth type requires; the allowed values are unverified and preserved as stored.", "critical", src, magicaSource),
			field("/rootBones", "MagicaCloth2 root bones", "The root-bone Transform reference list.", "MagicaCloth2 builds its particle chains from these roots.", "runtime_json_configuration", "instanceID values are Unity runtime identifiers with no stable meaning across sessions; preserve them unless the whole reference set is rebuilt.", "critical", src, magicaSource),
			field("/colliderCollisionConstraint", "MagicaCloth2 collider collision constraint", "The collider collision constraint block.", "MagicaCloth2 resolves cloth-versus-collider collision from this block.", "runtime_json_configuration", "Keep the mode and its referenced collider list coherent.", "high", src, magicaSource),
		}
	}

	magicaFields := func(src Source) []FieldPattern {
		return []FieldPattern{
			pattern("/{sourceRenderers,paintMaps,rootBones}", "MagicaCloth2 object reference list", "A Unity object reference list written by JsonUtility as instanceID records.", "MagicaCloth2 resolves these references when it builds the cloth solver.", "runtime_json_configuration", "instanceID values are runtime identifiers; preserve them unless the whole reference set is rebuilt.", src, magicaSource),
			pattern("/{clothType,paintMode,meshWriteMode,paintMapUvChannel,connectionMode,updateMode,normalAxis}", "MagicaCloth2 mode enum", "A ClothSerializeData mode or axis enum stored as an int32.", "MagicaCloth2 selects a code path from this value.", "runtime_json_configuration", "The allowed values are unverified; keep the stored value unless a same-build sample confirms another one.", src, magicaSource),
			pattern("/{gravity,gravityDirection,gravityFalloff,rotationalInterpolation,rootRotation,animationPoseRatio,stablizationTimeAfterReset,blendWeight}", "MagicaCloth2 global parameter", "A top-level scalar or vector simulation parameter.", "MagicaCloth2 applies these values globally to the cloth solver.", "runtime_json_configuration", "Use finite values coherent with the target model scale.", src, magicaSource),
			pattern("/{damping,radius}", "MagicaCloth2 curve parameter", "A value plus optional AnimationCurve pair.", "MagicaCloth2 evaluates the curve along the particle chain when useCurve is set.", "runtime_json_configuration", "Keep useCurve consistent with the curve you actually intend the solver to read.", src, magicaSource),
			pattern("/*Constraint/**", "MagicaCloth2 constraint member", "A member of one ClothSerializeData constraint block.", "MagicaCloth2 applies the constraint using these members.", "runtime_json_configuration", "Edit a constraint switch together with the parameters it enables.", src, magicaSource),
			pattern("/*/curve/m_Curve/*/{time,value,inSlope,outSlope,tangentMode,weightedMode,inWeight,outWeight}", "Unity keyframe", "One UnityEngine.Keyframe record inside a serialized AnimationCurve.", "Unity rebuilds the AnimationCurve from these keyframes.", "runtime_curve", "Keep keyframe times ordered and edit tangents conservatively.", src, magicaSource),
			pattern("/**/instanceID", "Unity instance reference", "A UnityEngine.Object reference written by JsonUtility, where 0 means a null reference.", "Unity resolves the reference at load time from the current scene.", "runtime_json_configuration", "These identifiers are not stable across sessions; preserve the stored value.", src, magicaSource),
		}
	}

	makeProfile := func(id, ext, title, summary, verificationNotes string, sources []Source, fields []Field, patterns []FieldPattern) Guide {
		g := guide(title, summary, FormatVerificationSerializationVerified, verificationNotes, sources, fields)
		g.FieldPatterns = patterns
		g.Invariants = []string{
			"The editing JSON root is the payload object itself; there is no surrounding envelope.",
			"The destination format is selected by the " + ext + " file name.",
			"The only supported wire format is an int32 length prefix followed by LZ4 Block Array-compressed MessagePack.",
			"A JSON null root represents a nil MessagePack root value.",
		}
		_ = id
		return g
	}

	dbconf := makeProfile("kces.dbconf", ".dbconf", "KCES .dbconf DynamicBone guide", "A DynamicBoneStatus configuration for DynamicYureBone.", "DynamicBoneMgr selects .dbconf for legacy dynamic-yure settings; DynamicYureBone reads the native status through a length-prefixed LZ4 MessagePack payload.", []Source{dynamicMgrSource, dynamicSource, payloadSource}, []Field{
		field("/version", "DynamicBoneStatus version", "The indexed DynamicBoneStatus version, currently 1000.", "DynamicYureBone reads this version before applying the status to its solver.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", dynamicSource, payloadSource),
	}, dynamicFields())
	dbconf.Rules = append(dbconf.Rules, Rule{ID: "dbconf-native-only", AppliesTo: []string{"/version"}, Severity: "error", Summary: "Only the native .dbconf wire is supported.", Details: "The supported form is int32-length-LZ4 MessagePack DynamicBoneStatus. KCES ExportCM writes direct Unity JSON with the same extension for COM3D2.5 to read; that export is a game-generated intermediate rather than a KCES resource and is rejected.", Evidence: []Source{payloadSource, dynamicMgrSource}})

	dbcol := makeProfile("kces.dbcol", ".dbcol", "KCES .dbcol collider guide", "A ColliderPackage for the dynamic-bone and limb collider state of DynamicYureBone.", "DynamicBoneMgr pairs .dbcol with .dbconf and DynamicYureBone applies the typed collider package to generated collider components.", []Source{dynamicMgrSource, dynamicSource, colliderSource, payloadSource}, []Field{
		field("/version", "Collider package version", "The indexed collider-package version.", "The loader reads this version before restoring collider statuses and limb toggles.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", colliderSource, payloadSource),
		field("/colliders", "Collider references", "The list of collider union entries.", "Each entry becomes one runtime collider component.", "runtime_collision", "Preserve list order and the nil-versus-empty distinction.", "critical", dynamicSource, colliderSource, payloadSource),
		field("/limbEnableList", "Limb enable states", "The optional limb type and enabled-flag list.", "The loader toggles generated limb colliders from this list.", "runtime_collision", "Preserve list order and the nil-versus-empty distinction.", "high", colliderSource, payloadSource),
	}, colliderFields())
	dbcol.Rules = append(dbcol.Rules, Rule{ID: "collider-union-coherence", AppliesTo: []string{"/colliders"}, Severity: "error", Summary: "Collider type tags and concrete objects must agree.", Details: "The decoder selects plane, capsule, sphere, or maid-property status from each type tag and rejects every unsupported tag.", Evidence: []Source{colliderSource}})
	dbcol.Rules = append(dbcol.Rules, Rule{ID: "dbcol-native-only", AppliesTo: []string{"/version"}, Severity: "error", Summary: "Only the native .dbcol wire is supported.", Details: "KCES ExportCM writes direct Unity JSON with the same extension for COM3D2.5 to read; that export is a game-generated intermediate rather than a KCES resource and is rejected.", Evidence: []Source{payloadSource, dynamicMgrSource}})

	db2conf := makeProfile("kces.db2conf", ".db2conf", "KCES .db2conf MagicaCloth2 ClothSerializeData guide", "A length-prefixed LZ4 MessagePack string containing the MagicaCloth2 ClothSerializeData Unity JSON document used by the newer DynamicYureBone MagicaCloth2 path.", "DynamicBoneMgr selects .db2conf for MagicaCloth2 dynamic-yure settings; the wire payload is a MessagePack string whose content is the typed ClothSerializeData document.", []Source{dynamicMgrSource, dynamicSource, payloadSource}, magicaRootFields(dynamicSource), magicaFields(dynamicSource))

	dsbconf := makeProfile("kces.dsbconf", ".dsbconf", "KCES .dsbconf skirt ClothParams guide", "A ClothParams object for the legacy MagicaCloth skirt simulation of DynamicKCESSkirtBone.", "DynamicBoneMgr selects .dsbconf for legacy skirt physics and DynamicKCESSkirtBone deserializes ClothParams from the length-prefixed LZ4 MessagePack payload.", []Source{dynamicMgrSource, skirtSource, clothSource, payloadSource}, clothRootFields(skirtSource), clothFields())
	dsb2conf := makeProfile("kces.dsb2conf", ".dsb2conf", "KCES .dsb2conf skirt MagicaCloth2 ClothSerializeData guide", "A length-prefixed LZ4 MessagePack string containing the MagicaCloth2 ClothSerializeData document for the newer skirt path.", "DynamicBoneMgr selects .dsb2conf for MagicaCloth2 skirts and DynamicKCESSkirtBone passes the decoded document to its MagicaCloth2 setup.", []Source{dynamicMgrSource, skirtSource, payloadSource}, magicaRootFields(skirtSource), magicaFields(skirtSource))

	dslconf := makeProfile("kces.dslconf", ".dslconf", "KCES .dslconf sleeve ClothParams guide", "A ClothParams object for the legacy MagicaCloth sleeve simulation of DynamicSleeveBone.", "DynamicBoneMgr selects .dslconf for sleeve physics and DynamicSleeveBone applies the typed ClothParams object.", []Source{dynamicMgrSource, sleeveSource, clothSource, payloadSource}, clothRootFields(sleeveSource), clothFields())
	dsl2conf := makeProfile("kces.dsl2conf", ".dsl2conf", "KCES .dsl2conf sleeve MagicaCloth2 ClothSerializeData guide", "A length-prefixed LZ4 MessagePack string containing the MagicaCloth2 ClothSerializeData document for the newer sleeve path.", "DynamicBoneMgr selects .dsl2conf for MagicaCloth2 sleeve parameters and DynamicSleeveBone forwards the decoded document to its MagicaCloth2 setup.", []Source{dynamicMgrSource, sleeveSource, payloadSource}, magicaRootFields(sleeveSource), magicaFields(sleeveSource))

	dslcol := makeProfile("kces.dslcol", ".dslcol", "KCES .dslcol sleeve collider guide", "A ColliderPackage for the cloth colliders of DynamicSleeveBone.", "DynamicBoneMgr pairs .dslcol with .dslconf or .dsl2conf and DynamicSleeveBone reconstructs collider components from the common collider package.", []Source{dynamicMgrSource, sleeveSource, colliderSource, payloadSource}, []Field{
		field("/version", "Collider package version", "The indexed collider-package version.", "The loader reads this version before restoring collider statuses and limb toggles.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", colliderSource, payloadSource),
		field("/colliders", "Collider references", "The list of collider union entries.", "Each entry becomes one runtime collider component.", "runtime_collision", "Preserve list order and the nil-versus-empty distinction.", "critical", dynamicSource, colliderSource, payloadSource),
		field("/limbEnableList", "Limb enable states", "The optional limb type and enabled-flag list.", "The loader toggles generated limb colliders from this list.", "runtime_collision", "Preserve list order and the nil-versus-empty distinction.", "high", colliderSource, payloadSource),
	}, colliderFields())
	dslcol.Rules = append(dslcol.Rules, Rule{ID: "dslcol-native-only", AppliesTo: []string{"/version"}, Severity: "error", Summary: "Only the native .dslcol wire is supported.", Details: "KCES ExportCM writes this extension as UTF-8 Unity JSON wrapped in a BinaryWriter string for COM3D2.5 to read; that export is a game-generated intermediate rather than a KCES resource and is rejected.", Evidence: []Source{payloadSource, sleeveSource}})

	ikcol := makeProfile("kces.ikcol", ".ikcol", "KCES .ikcol IK collider guide", "An IKColliderPackage used by full-body IK to restore collider groups for each IK effector.", "IKColliderSaveLoader reads .ikcol as a length-prefixed LZ4 MessagePack package and creates native collider components for each effector group.", []Source{ikSource, colliderSource, payloadSource}, []Field{
		field("/version", "IK collider package version", "The indexed IK collider-package version.", "IKColliderSaveLoader reads this version before creating effector collider objects.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", ikSource, payloadSource),
		field("/groups", "IK effector groups", "The per-effector collider group list.", "IKColliderSaveLoader populates each full-body IK effector collider list from these groups.", "runtime_ik_collision", "Keep effector targets and collider geometry aligned with the target rig.", "critical", ikSource, payloadSource),
	}, ikFields())
	ikcolBytes := makeProfile("kces.ikcol.bytes", ".ikcol.bytes", "KCES .ikcol.bytes IK collider guide", "The compound-extension Unity resource form of .ikcol. Its wire payload is identical to the standalone IK collider file.", "The default IKColliderSaveLoader resource name is ik_collider.ikcol.bytes; the complete compound extension must be preserved when dispatching this payload.", []Source{ikSource, colliderSource, payloadSource}, []Field{
		field("/version", "IK collider package version", "The indexed IK collider-package version.", "IKColliderSaveLoader reads this version before creating effector collider objects.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", ikSource, payloadSource),
		field("/groups", "IK effector groups", "The per-effector collider group list.", "IKColliderSaveLoader populates each full-body IK effector collider list from these groups.", "runtime_ik_collision", "Keep effector targets and collider geometry aligned with the target rig.", "critical", ikSource, payloadSource),
	}, ikFields())

	limbcol := makeProfile("kces.limbcol", ".limbcol", "KCES .limbcol limb collider guide", "A LimbColliderPackage used by LimbColliderMgr to restore generated arm and leg collider statuses.", "LimbColliderMgr loads limbconf.limbcol as a length-prefixed LZ4 MessagePack package and applies each target status to the corresponding limb collider.", []Source{limbSource, colliderSource, payloadSource}, []Field{
		field("/version", "Limb collider package version", "The indexed limb collider-package version.", "LimbColliderMgr reads this version before applying limb collider statuses.", "version_marker", "Preserve the stored version; other layouts are unsupported.", "critical", limbSource, payloadSource),
		field("/items", "Limb collider items", "The limb target and collider status list.", "LimbColliderMgr looks up each limb enum and calls SetStatus on its generated collider.", "runtime_limb_collision", "Keep target enum values and collider geometry coherent with the target body.", "critical", limbSource, payloadSource),
	}, limbFields())

	return map[string]Guide{
		"kces.dbconf":      dbconf,
		"kces.dbcol":       dbcol,
		"kces.db2conf":     db2conf,
		"kces.dsbconf":     dsbconf,
		"kces.dsb2conf":    dsb2conf,
		"kces.dslconf":     dslconf,
		"kces.dsl2conf":    dsl2conf,
		"kces.dslcol":      dslcol,
		"kces.ikcol":       ikcol,
		"kces.ikcol.bytes": ikcolBytes,
		"kces.limbcol":     limbcol,
	}
}
