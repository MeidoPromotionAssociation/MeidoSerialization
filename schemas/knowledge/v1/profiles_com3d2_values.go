package knowledgev1

const (
	com3d2MPN248ValueSetID         = "com3d2.mpn.2_48"
	com3d2MPN348ValueSetID         = "com3d2.mpn.3_48"
	com3d2SlotID248ValueSetID      = "com3d2.tbody_slot_id.2_48"
	com3d2SlotID348ValueSetID      = "com3d2.tbody_slot_id.3_48"
	com3d2PartsColorValueSetID     = "com3d2.maid_parts_color"
	com3d2SystemMaterialValueSetID = "com3d2.system_material"
	com3d2TargetBodyTypeValueSetID = "com3d2.target_body_type.3_48"
	com3d2MeshMorphTagValueSetID   = "com3d2.mesh_morph_tag.3_48"
	com3d2ChikubiStateValueSetID   = "com3d2.chikubi_state.3_48"
	com3d2ChinkoStateValueSetID    = "com3d2.chinko_state.3_48"
)

// com3d2MenuValueSets 构建两个已审核游戏版本的菜单枚举和值名称目录
// com3d2MenuValueSets builds menu enum and value-name catalogs for both reviewed game versions
func com3d2MenuValueSets() []ValueSet {
	mpn248Source := source("COM3D2 2.48.0", "COM3D2 2.48.0/Assembly-CSharp/MPN.cs", "MPN", 3, 138, "Defines the numeric MPN ordering used by the reviewed COM3D2 2.48 menu compiler and runtime.")
	mpn348Source := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/MPN.cs", "MPN", 3, 239, "Defines the expanded and reordered numeric MPN set used by the reviewed COM3D2_5 3.48 runtime.")
	slot248Source := source("COM3D2 2.48.0", "COM3D2 2.48.0/Assembly-CSharp/TBody.cs", "TBody.SlotID", 3677, 3738, "Defines model and attachment slot names for the reviewed COM3D2 2.48 runtime.")
	slot348Source := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/TBody.cs", "TBody.SlotID", 5644, 5720, "Defines the expanded model and attachment slot names for the reviewed COM3D2_5 3.48 runtime.")
	partsColor248Source := source("COM3D2 2.48.0", "COM3D2 2.48.0/Assembly-CSharp/MaidParts.cs", "MaidParts.PARTS_COLOR", 231, 248, "Defines infinite-color channel names and numeric values used by tex commands.")
	partsColor348Source := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/MaidParts.cs", "MaidParts.PARTS_COLOR", 232, 249, "Retains the reviewed infinite-color channel ordering in COM3D2_5 3.48.")
	systemMaterial248Source := source("COM3D2 2.48.0", "COM3D2 2.48.0/Assembly-CSharp/GameUty.cs", "GameUty.SystemMaterial", 1295, 1302, "Defines system blend materials accepted by texture-composition commands.")
	systemMaterial348Source := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/GameUty.cs", "GameUty.SystemMaterial", 1564, 1571, "Retains the reviewed system blend-material ordering in COM3D2_5 3.48.")
	bodyStateSource := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/TBodySkin.cs", "TBodySkin body-state enums", 2164, 2193, "Defines nipple, penis, and target-body enum values used by COM3D2_5-only menu commands.")
	meshMorphSource := source("COM3D2_5 3.48.0", "COM3D2_5 3.48.0/Assembly-CSharp/TMorphSkin.cs", "TMorphSkin.BaseBlendValue.Tag", 1833, 1838, "Defines the mesh-morph base-blend tags used by the COM3D2_5 meshmorph command.")

	return []ValueSet{
		{
			ID:           com3d2MPN248ValueSetID,
			CSharpType:   "MPN",
			Description:  "All MPN names and their zero-based numeric values in COM3D2 2.48.0.",
			EditGuidance: "Use this set only for a 2.48 target. Enum-parsing branches differ in case handling, so use the exact declared name unless the selected command form documents normalization; numeric values are build-specific.",
			ReviewedIn:   []string{"COM3D2 2.48.0"},
			Values:       sequentialValueSetValues(com3d2MPN248Names),
			Evidence:     []Source{mpn248Source},
		},
		{
			ID:           com3d2MPN348ValueSetID,
			CSharpType:   "MPN",
			Description:  "All MPN names and their zero-based numeric values in COM3D2_5 3.48.0.",
			EditGuidance: "Use this set for a 3.48 target and preserve exact declared spelling unless the selected command form documents normalization. Do not reuse 2.48 numeric MPN values: the enum was expanded and reordered.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues(com3d2MPN348Names),
			Evidence:     []Source{mpn348Source},
		},
		{
			ID:           com3d2SlotID248ValueSetID,
			CSharpType:   "TBody.SlotID",
			Description:  "All TBody slot names and their zero-based numeric values in COM3D2 2.48.0, including the end sentinel.",
			EditGuidance: "Use a non-sentinel name that exists on the target body. Preserve the exact stored spelling when a command does not explicitly request case-insensitive enum parsing.",
			ReviewedIn:   []string{"COM3D2 2.48.0"},
			Values:       sequentialValueSetValues(com3d2SlotID248Names),
			Evidence:     []Source{slot248Source},
		},
		{
			ID:           com3d2SlotID348ValueSetID,
			CSharpType:   "TBody.SlotID",
			Description:  "All TBody slot names and their zero-based numeric values in COM3D2_5 3.48.0, including added split slots and the end sentinel.",
			EditGuidance: "Use this set for a 3.48 target. New entries such as hairS_2 and accHead_2 are not available in the reviewed 2.48 build.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues(com3d2SlotID348Names),
			Evidence:     []Source{slot348Source},
		},
		{
			ID:           com3d2PartsColorValueSetID,
			CSharpType:   "MaidParts.PARTS_COLOR",
			Description:  "Infinite-color channel names used by tex and color-set metadata in both reviewed builds.",
			EditGuidance: "NONE means no color binding and MAX is a sentinel; use a concrete channel for editable infinite-color textures.",
			ReviewedIn:   []string{"COM3D2 2.48.0", "COM3D2_5 3.48.0"},
			Values: []ValueSetValue{
				{Name: "NONE", Number: -1},
				{Name: "EYE_L", Number: 0},
				{Name: "EYE_R", Number: 1},
				{Name: "HAIR", Number: 2},
				{Name: "EYE_BROW", Number: 3},
				{Name: "UNDER_HAIR", Number: 4},
				{Name: "SKIN", Number: 5},
				{Name: "NIPPLE", Number: 6},
				{Name: "HAIR_OUTLINE", Number: 7},
				{Name: "SKIN_OUTLINE", Number: 8},
				{Name: "EYE_WHITE", Number: 9},
				{Name: "MATSUGE_UP", Number: 10},
				{Name: "MATSUGE_LOW", Number: 11},
				{Name: "FUTAE", Number: 12},
				{Name: "MAX", Number: 13},
			},
			Evidence: []Source{partsColor248Source, partsColor348Source},
		},
		{
			ID:           com3d2SystemMaterialValueSetID,
			CSharpType:   "GameUty.SystemMaterial",
			Description:  "System materials used as texture-composition blend modes in both reviewed builds.",
			EditGuidance: "Use Alpha, Multiply, InfinityColor, or TexTo8bitTex. Max is the enum sentinel and is not a material resource.",
			ReviewedIn:   []string{"COM3D2 2.48.0", "COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues([]string{"Alpha", "Multiply", "InfinityColor", "TexTo8bitTex", "Max"}),
			Evidence:     []Source{systemMaterial248Source, systemMaterial348Source},
		},
		{
			ID:           com3d2TargetBodyTypeValueSetID,
			CSharpType:   "TBodySkin.TargetBodyType",
			Description:  "Body-type selectors passed to COM3D2_5 CRC additem loading.",
			EditGuidance: "Use None unless the model is explicitly authored for Woman or Man.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues([]string{"None", "Woman", "Man"}),
			Evidence:     []Source{bodyStateSource},
		},
		{
			ID:           com3d2MeshMorphTagValueSetID,
			CSharpType:   "TMorphSkin.BaseBlendValue.Tag",
			Description:  "Base-blend tag names accepted by the reviewed COM3D2_5 meshmorph branch.",
			EditGuidance: "Use a concrete tag; MAX is the enum sentinel.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues([]string{"パンツ", "靴下", "MAX"}),
			Evidence:     []Source{meshMorphSource},
		},
		{
			ID:           com3d2ChikubiStateValueSetID,
			CSharpType:   "TBodySkin.CHIKUBI_STATE",
			Description:  "Nipple-state enum values accepted by the reviewed COM3D2_5 body-state command.",
			EditGuidance: "Choose a state supported by the target CRC body model.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues([]string{"None", "固定凸", "基本凹"}),
			Evidence:     []Source{bodyStateSource},
		},
		{
			ID:           com3d2ChinkoStateValueSetID,
			CSharpType:   "TBodySkin.CHINKO_STATE",
			Description:  "Penis-state enum values accepted by the reviewed COM3D2_5 body-state command.",
			EditGuidance: "Choose a state supported by the target CRC body model.",
			ReviewedIn:   []string{"COM3D2_5 3.48.0"},
			Values:       sequentialValueSetValues([]string{"None", "しまう"}),
			Evidence:     []Source{bodyStateSource},
		},
	}
}

// sequentialValueSetValues 按名称切片顺序生成从零开始的精确数值映射
// sequentialValueSetValues generates exact zero-based numeric mappings in name-slice order
func sequentialValueSetValues(names []string) []ValueSetValue {
	values := make([]ValueSetValue, len(names))
	for index, name := range names {
		values[index] = ValueSetValue{Name: name, Number: index}
	}
	return values
}

// appendValueSetSources 将值集合证据去重后追加到指南顶层来源
// appendValueSetSources appends deduplicated value-set evidence to guide-level sources
func appendValueSetSources(sources []Source, valueSets []ValueSet) []Source {
	seen := make(map[string]struct{}, len(sources))
	for _, evidence := range sources {
		seen[valueSetSourceKey(evidence)] = struct{}{}
	}
	for _, valueSet := range valueSets {
		for _, evidence := range valueSet.Evidence {
			key := valueSetSourceKey(evidence)
			if _, exists := seen[key]; exists {
				continue
			}
			sources = append(sources, evidence)
			seen[key] = struct{}{}
		}
	}
	return sources
}

// valueSetSourceKey 为源码证据生成用于去重的稳定复合键
// valueSetSourceKey creates a stable composite key used to deduplicate source evidence
func valueSetSourceKey(evidence Source) string {
	return evidence.Kind + "\x00" + evidence.GameVersion + "\x00" + evidence.Path + "\x00" + evidence.Symbol
}

var com3d2MPN248Names = []string{
	"null_mpn", "MuneL", "MuneS", "MuneTare", "RegFat", "ArmL", "Hara", "RegMeet", "KubiScl", "UdeScl",
	"EyeScl", "EyeSclX", "EyeSclY", "EyePosX", "EyePosY", "EyeClose", "EyeBallPosX", "EyeBallPosY", "EyeBallSclX", "EyeBallSclY",
	"EarNone", "EarElf", "EarRot", "EarScl", "NosePos", "NoseScl", "FaceShape", "FaceShapeSlim", "MayuShapeIn", "MayuShapeOut",
	"MayuX", "MayuY", "MayuRot", "HeadX", "HeadY", "DouPer", "sintyou", "koshi", "kata", "west",
	"MuneUpDown", "MuneYori", "MuneYawaraka", "MayuThick", "MayuLong", "Yorime", "MabutaUpIn", "MabutaUpIn2", "MabutaUpMiddle", "MabutaUpOut",
	"MabutaUpOut2", "MabutaLowIn", "MabutaLowUpMiddle", "MabutaLowUpOut", "body", "moza", "head", "hairf", "hairr", "hairt",
	"hairs", "hairaho", "haircolor", "skin", "acctatoo", "accnail", "underhair", "hokuro", "mayu", "lip",
	"eye", "eye_hi", "eye_hi_r", "chikubi", "chikubicolor", "eyewhite", "nose", "facegloss", "matsuge_up", "matsuge_low",
	"futae", "wear", "skirt", "mizugi", "bra", "panz", "stkg", "shoes", "headset", "glove",
	"acchead", "accha", "acchana", "acckamisub", "acckami", "accmimi", "accnip", "acckubi", "acckubiwa", "accheso",
	"accude", "accashi", "accsenaka", "accshippo", "accanl", "accvag", "megane", "accxxx", "handitem", "acchat",
	"onepiece", "set_maidwear", "set_mywear", "set_underwear", "set_body", "set_head_slider", "folder_eye", "folder_mayu", "folder_underhair", "folder_skin",
	"folder_eyewhite", "folder_matsuge_up", "folder_matsuge_low", "folder_futae", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara", "seieki_face", "seieki_mune",
	"seieki_hip", "seieki_ude", "seieki_ashi",
}

var com3d2MPN348Names = []string{
	"null_mpn", "RegFat", "ArmL", "Hara", "RegMeet", "KubiScl", "UdeScl", "DouPer", "sintyou", "koshi",
	"kata", "west", "MuneL", "MuneS", "MuneM", "MuneTare", "MuneUpDown", "MuneYori", "MuneYawaraka", "MunePosX",
	"MunePosY", "MuneThick", "MuneLong", "MuneDir", "DouThick1X", "DouThick1Y", "DouThick2X", "DouThick2Y", "DouThick3X", "DouThick3Y",
	"ShoulderThick", "UpperArmThickX", "UpperArmThickY", "LowerArmThickX", "LowerArmThickY", "ElbowThickX", "ElbowThickY", "NeckThickX", "NeckThickY", "HandSize",
	"DouThick4X", "DouThick4Y", "DouThick5X", "DouThick5Y", "WaistPos", "HipSize", "HipRot", "ThighThickX", "ThighThickY", "KneeThickX",
	"KneeThickY", "CalfThickX", "CalfThickY", "AnkleThickX", "AnkleThickY", "FootSize", "UpperArmLowerThickX", "UpperArmLowerThickY", "WristThickX", "WristThickY",
	"ClavicleThick", "ShoulderTension", "ThighLowerThickX", "ThighLowerThickY", "ThighShin", "HaraN", "ChikubiH", "ChikubiK1", "ChikubiK2", "ChikubiK2_MuneS",
	"ChikubiR", "ChikubiW", "Nyurin1", "Nyurin2", "Nyurin3", "Nyurin4", "Nyurin5", "Nyurin6", "Nyurin7", "Nyurin8",
	"ChikubiWearTotsu", "MuscleSkin", "HeadX", "HeadY", "FaceShape", "FaceShapeSlim", "EyeScl", "EyeSclX", "EyeSclY", "EyePosX",
	"EyePosY", "EyeClose", "EyeBallPosX", "EyeBallPosY", "EyeBallSclX", "EyeBallSclY", "EarNone", "EarElf", "EarRot", "EarScl",
	"NosePos", "NoseScl", "MayuShapeIn", "MayuShapeOut", "MayuX", "MayuY", "MayuRot", "MayuThick", "MayuLong", "Yorime",
	"MabutaUpIn", "MabutaUpIn2", "MabutaUpMiddle", "MabutaUpOut", "MabutaUpOut2", "MabutaLowIn", "MabutaLowUpMiddle", "MabutaLowUpOut", "Eyedel", "Itome",
	"Ha1", "Ha2", "Ha3", "Ha4", "Ha5", "Ha6", "FutaePosX", "FutaePosY", "FutaeRot", "HitomiHiPosX",
	"HitomiHiPosY", "HitomiHiSclY", "HitomiShapeUp", "HitomiShapeLow", "HitomiShapeIn", "HitomiShapeOutUp", "HitomiShapeOutLow", "HitomiRot", "HohoShape", "LipThick",
	"WearSuso", "KuikomiPants", "KuikomiStkg", "Hanasuji", "Washibana", "body", "moza", "head", "hairf", "hairr",
	"hairt", "hairs", "hairaho", "haircolor", "skin", "acctatoo", "accnail", "underhair", "asshair", "hokuro",
	"mayu", "lip", "chikubi", "chikubicolor", "eye", "eye_hi", "eye_hi_r", "eyewhite", "nose", "facegloss",
	"matsuge_up", "matsuge_low", "futae", "wear", "skirt", "mizugi", "mizugi_top", "mizugi_buttom", "bra", "panz",
	"slip", "stkg", "shoes", "headset", "glove", "acchead", "accha", "acchana", "accface", "acckamisub",
	"acckami", "accmimi", "accnip", "acckubi", "acckubiwa", "accheso", "accude", "accashi", "accsenaka", "accshippo",
	"acckoshi", "accanl", "accvag", "megane", "accxxx", "handitem", "acchat", "onepiece", "jacket", "vest",
	"shirt", "set_maidwear", "set_mywear", "set_underwear", "set_body", "set_face", "set_head_slider", "folder_eye", "folder_mayu", "folder_underhair",
	"folder_skin", "folder_eyewhite", "folder_matsuge_up", "folder_matsuge_low", "folder_futae", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara", "seieki_face",
	"seieki_mune", "seieki_hip", "seieki_ude", "seieki_ashi",
}

var com3d2SlotID248Names = []string{
	"body", "head", "eye", "hairF", "hairR", "hairS", "hairT", "wear", "skirt", "onepiece",
	"mizugi", "panz", "bra", "stkg", "shoes", "headset", "glove", "accHead", "hairAho", "accHana",
	"accHa", "accKami_1_", "accMiMiR", "accKamiSubR", "accNipR", "HandItemR", "accKubi", "accKubiwa", "accHeso", "accUde",
	"accAshi", "accSenaka", "accShippo", "accAnl", "accVag", "kubiwa", "megane", "accXXX", "chinko", "chikubi",
	"accHat", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara", "seieki_face", "seieki_mune", "seieki_hip", "seieki_ude", "seieki_ashi",
	"accNipL", "accMiMiL", "accKamiSubL", "accKami_2_", "accKami_3_", "HandItemL", "underhair", "moza", "end",
}

var com3d2SlotID348Names = []string{
	"body", "head", "eye", "hairF", "hairR", "hairS", "hairS_2", "hairT", "hairT_2", "wear",
	"skirt", "onepiece", "mizugi", "mizugi_top", "mizugi_buttom", "panz", "slip", "bra", "stkg", "shoes",
	"headset", "glove", "jacket", "vest", "shirt", "accHead", "accHead_2", "hairAho", "accHana", "accHa",
	"accKami_1_", "accMiMiR", "accKamiSubR", "accNipR", "HandItemR", "accKubi", "accKubiwa", "accHeso", "accUde", "accUde_2",
	"accAshi", "accAshi_2", "accSenaka", "accShippo", "accKoshi", "accAnl", "accVag", "kubiwa", "megane", "accXXX",
	"chinko", "chikubi", "accFace", "accHat", "accHat_2", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara", "seieki_face",
	"seieki_mune", "seieki_hip", "seieki_ude", "seieki_ashi", "accNipL", "accMiMiL", "accKamiSubL", "accKami_2_", "accKami_3_", "HandItemL",
	"underhair", "asshair", "moza", "end",
}
