package knowledgev1

import "strconv"

// KCES2 菜单命令引用的值集标识符
// Value-set identifiers referenced by KCES2 menu commands
const (
	kces2SlotIDValueSetID         = "kces2.tbody_slot_id.1_36_0"
	kces2MPNValueSetID            = "kces2.mpn.1_36_0"
	kces2PartsColorValueSetID     = "kces2.maid_infinity_color.parts_color"
	kces2SystemMaterialValueSetID = "kces2.game_utility.system_material"
	kces2AlphaTypeValueSetID      = "kces2.material_mgr.alpha_type"
	kces2ChikubiStateValueSetID   = "kces2.tbody_skin.chikubi_state"
	kces2ChinkoStateValueSetID    = "kces2.tbody_skin.chinko_state"
	kces2MeshMorphTagValueSetID   = "kces2.tmorph_skin.base_blend_tag"
	kces2TargetBodyTypeValueSetID = "kces2.menu.target_body_type"
	kces2DefineValueSetID         = "kces2.menu.define"
	kces2AttributeValueSetID      = "kces2.menu.attribute"
	kces2MoveHideModeValueSetID   = "kces2.tbody.move_hide_mode"
	kces2PartHideTypeValueSetID   = "kces2.tbody.part_hide_type"
	kces2HaraYureValueSetID       = "kces2.menu.hara_yure_limit"
	kces2HairSearchTypeValueSetID = "kces2.hair_length_ctrl.search_type"
)

// kces2MenuValueSets 构建 KCES2 1.36.0 菜单命令引用的全部枚举值集
// kces2MenuValueSets builds every enum value set referenced by KCES2 1.36.0 menu commands
func kces2MenuValueSets() []ValueSet {
	slotIDSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/TBody.cs", "TBody.SlotID", 4207, 4356, "Defines the none sentinel, body, head, hair, wear, accessory, hand-item, and end sentinel slot names, followed by 72 numbered accessory slots.")
	mpnSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/MPN.cs", "MPN", 3, 337, "Defines the numeric MPN ordering used by maid-property and item-menu commands in KCES2 1.36.0.")
	partsColorSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/MaidInfinityColor.cs", "MaidInfinityColor.PARTS_COLOR", 431, 457, "Defines infinite-color channel names and numeric values used by tex, partcolor, and composition commands.")
	systemMaterialSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/Scourt/Utility/GameUtility.cs", "GameUtility.SystemMaterial", 140, 153, "Defines the system blend materials accepted by texture-composition commands.")
	alphaTypeSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/MaterialMgr.cs", "MaterialMgr.ALPHA_TYPE", 2095, 2100, "Defines the alpha-source selectors used by alpha-annotated texture commands.")
	bodyStateSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/TBodySkin.cs", "TBodySkin body-state enums", 1528, 1539, "Defines nipple and penis state values used by the body-state menu commands.")
	meshMorphSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/TMorphSkin.cs", "TMorphSkin.BaseBlendValue.Tag", 2284, 2289, "Defines the base-blend tags used by the meshmorph command.")
	menuEnumSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/Parts/Menu.cs", "Parts.Menu enums", 595, 628, "Defines DEFINE flags, target body types, attributes, and belly-sway limits on Parts.Menu.")
	bodyFlagSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/TBody.cs", "TBody.MOVE_HIDE_MODE and PART_HIDE_TYPE", 4364, 4376, "Defines move/hide flag modes and part-hide types used by the parthidemove command.")
	hairSearchSource := source("KCES2 1.36.0", "KCES2 1.36.0/Assembly-CSharp/HairLengthCtrl.cs", "HairLengthCtrl.SearchAndAddHairLengthTarget", 44, 66, "Selects FIRST_BROTHER for fbrother, FIRST_CHILD for fchild, and ALL for all; any other value raises an assertion.")

	return []ValueSet{
		{
			ID:           kces2SlotIDValueSetID,
			CSharpType:   "TBody.SlotID",
			Description:  "All TBody slot names and their numeric values in KCES2 1.36.0, including the -1 none sentinel and the end sentinel.",
			EditGuidance: "Use a non-sentinel name that exists on the target body. Slot parsing in the interpreter uses case-insensitive Enum.Parse, but keep the declared spelling for portability.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       kces2SlotIDValues(),
			Evidence:     []Source{slotIDSource},
		},
		{
			ID:           kces2MPNValueSetID,
			CSharpType:   "MPN",
			Description:  "All 337 MPN names and their zero-based numeric values in KCES2 1.36.0.",
			EditGuidance: "Parse.TryParse resolves names case-insensitively, but prop and setslotitem use Maid.SetProp's case-sensitive Enum.Parse; prefer exact declared spelling. Numeric values are build-specific.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues(kces2MPNNames),
			Evidence:     []Source{mpnSource},
		},
		{
			ID:           kces2PartsColorValueSetID,
			CSharpType:   "MaidInfinityColor.PARTS_COLOR",
			Description:  "Infinite-color channel names used by tex, partcolor, partcolorrgb, and kimono-texture commands in KCES2 1.36.0.",
			EditGuidance: "NONE means no color binding and MAX is a sentinel; use a concrete channel for editable infinite-color textures.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       kces2PartsColorValues(),
			Evidence:     []Source{partsColorSource},
		},
		{
			ID:           kces2SystemMaterialValueSetID,
			CSharpType:   "GameUtility.SystemMaterial",
			Description:  "System materials used as texture-composition blend modes in KCES2 1.36.0.",
			EditGuidance: "Max is the enum sentinel and is not a material resource. The enum name is parsed case-insensitively through Parse.TryParse.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"Alpha", "BlendSelf", "Multiply", "InfinityColor", "InfinityColorPart", "InfinityColorGrada", "TexTo8bitTex", "AddNormal", "AlphaDstAlpha", "Screen", "Max"}),
			Evidence:     []Source{systemMaterialSource},
		},
		{
			ID:           kces2AlphaTypeValueSetID,
			CSharpType:   "MaterialMgr.ALPHA_TYPE",
			Description:  "Alpha-source selectors attached to a color definition as NAME:ALPHA_TYPE=PERCENT.",
			EditGuidance: "The runtime parses the selector case-insensitively; an unknown selector logs an error and falls back to ALPHA_NONE.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"ALPHA_NONE", "ALPHA_TEX", "ALPHA_MAT"}),
			Evidence:     []Source{alphaTypeSource},
		},
		{
			ID:           kces2ChikubiStateValueSetID,
			CSharpType:   "TBodySkin.CHIKUBI_STATE",
			Description:  "Nipple-state enum values accepted by the KCES2 body-state command.",
			EditGuidance: "Choose a state supported by the target CRC body model; parsing is case-insensitive.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"None", "固定凸", "基本凹"}),
			Evidence:     []Source{bodyStateSource},
		},
		{
			ID:           kces2ChinkoStateValueSetID,
			CSharpType:   "TBodySkin.CHINKO_STATE",
			Description:  "Penis-state enum values accepted by the KCES2 body-state command.",
			EditGuidance: "Choose a state supported by the target CRC body model; parsing is case-insensitive.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"None", "しまう"}),
			Evidence:     []Source{bodyStateSource},
		},
		{
			ID:           kces2MeshMorphTagValueSetID,
			CSharpType:   "TMorphSkin.BaseBlendValue.Tag",
			Description:  "Base-blend tag names accepted by the KCES2 meshmorph command.",
			EditGuidance: "Use a concrete tag; MAX is the enum sentinel and the interpreter parses the tag case-insensitively.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"パンツ", "靴下", "MAX"}),
			Evidence:     []Source{meshMorphSource},
		},
		{
			ID:           kces2TargetBodyTypeValueSetID,
			CSharpType:   "Menu.TargetBodyType",
			Description:  "Body-type selectors stored on a Parts.Menu and passed to additem model loading.",
			EditGuidance: "Use None unless the model is explicitly authored for Woman or Man.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"None", "Woman", "Man"}),
			Evidence:     []Source{menuEnumSource},
		},
		{
			ID:           kces2DefineValueSetID,
			CSharpType:   "Menu.DEFINE",
			Description:  "Flag combinations stored in defineTagNames/defineFirst and matched by ifdef conditions.",
			EditGuidance: "The ifdef branch parses the flag name case-sensitively; combine flags by numeric OR in the stored value.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values: []ValueSetValue{
				{Name: "NONE", Number: 0},
				{Name: "COLOR_MAMA", Number: 1},
				{Name: "COLOR_MUGEN", Number: 2},
				{Name: "COLOR_BUBUN", Number: 4},
				{Name: "COLOR_GRADA", Number: 8},
			},
			Evidence: []Source{menuEnumSource},
		},
		{
			ID:           kces2AttributeValueSetID,
			CSharpType:   "Menu.Attribute",
			Description:  "Attribute flags stored on a Parts.Menu and combined by bitwise OR.",
			EditGuidance: "Store the combined numeric value; individual names are only human-readable flags.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values: []ValueSetValue{
				{Name: "None", Number: 0},
				{Name: "WomanReccomend", Number: 1},
				{Name: "ManReccomend", Number: 2},
				{Name: "ManSuits", Number: 4},
				{Name: "NoExpressionFace", Number: 8},
				{Name: "NoMoveTatooHokuro", Number: 16},
			},
			Evidence: []Source{menuEnumSource},
		},
		{
			ID:           kces2MoveHideModeValueSetID,
			CSharpType:   "TBody.MOVE_HIDE_MODE",
			Description:  "Move/hide mode flags parsed from the ampersand-separated parthidemove mode token.",
			EditGuidance: "The interpreter uses case-sensitive Enum.Parse, so write MOVE and HIDE exactly as declared; combine them with & when both apply.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values: []ValueSetValue{
				{Name: "NONE", Number: 0},
				{Name: "MOVE", Number: 1},
				{Name: "HIDE", Number: 2},
			},
			Evidence: []Source{bodyFlagSource},
		},
		{
			ID:           kces2PartHideTypeValueSetID,
			CSharpType:   "TBody.PART_HIDE_TYPE",
			Description:  "Part-hide strategy names used by the parthidemove command.",
			EditGuidance: "Choose the mechanism matching the authored mesh: slot visibility or bone weighting.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"TYPE_SLOT_VISIBLE", "TYPE_BONE_WEIGHT"}),
			Evidence:     []Source{bodyFlagSource},
		},
		{
			ID:           kces2HaraYureValueSetID,
			CSharpType:   "Menu.HaraYureLimitType",
			Description:  "Belly-sway availability states copied onto the slot after additem.",
			EditGuidance: "Keep None unless the model requires an explicit availability override.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"None", "YureAvailable", "YureDisable"}),
			Evidence:     []Source{menuEnumSource},
		},
		{
			ID:           kces2HairSearchTypeValueSetID,
			CSharpType:   "HairLengthCtrl.SerchMode selector",
			Description:  "Bone-search selector strings accepted by the length command.",
			EditGuidance: "The interpreter compares the token case-sensitively and asserts on any other value; the bone name supports * as a regular-expression wildcard.",
			ReviewedIn:   []string{"KCES2 1.36.0"},
			Values:       sequentialValueSetValues([]string{"fbrother", "fchild", "all"}),
			Evidence:     []Source{hairSearchSource},
		},
	}
}

// kces2SlotIDValues 生成 TBody.SlotID 的精确数值映射；none 为 -1，其后名字从 0 开始递增
// kces2SlotIDValues builds the exact numeric mapping for TBody.SlotID; none is -1 and later names count up from 0
func kces2SlotIDValues() []ValueSetValue {
	names := []string{
		"none", "body", "head", "eye", "hairF", "hairR", "hairS", "hairS_2", "hairT", "hairT_2",
		"wear", "skirt", "onepiece", "mizugi", "mizugi_top", "mizugi_buttom", "panz", "slip", "bra", "stkg",
		"shoes", "headset", "glove", "jacket", "vest", "shirt", "accHead", "accHead_2", "hairAho", "accHana",
		"accHa", "accKami_1_", "accMiMiR", "accKamiSubR", "accNipR", "HandItemR", "accKubi", "accKubiwa", "accHeso", "accUde",
		"accUde_2", "accAshi", "accAshi_2", "accSenaka", "accShippo", "accKoshi", "accAnl", "accVag", "kubiwa", "megane",
		"accXXX", "chinko", "chikubi", "accFace", "accHat", "accHat_2", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara",
		"seieki_face", "seieki_mune", "seieki_hip", "seieki_ude", "seieki_ashi", "accNipL", "accMiMiL", "accKamiSubL", "accKami_2_", "accKami_3_",
		"HandItemL", "underhair", "asshair", "moza", "end",
	}
	for index := 1; index <= 72; index++ {
		names = append(names, "accAcc"+strconv.Itoa(index))
	}
	values := make([]ValueSetValue, 0, len(names))
	values = append(values, ValueSetValue{Name: "none", Number: -1})
	for index := 1; index < len(names); index++ {
		values = append(values, ValueSetValue{Name: names[index], Number: index - 1})
	}
	return values
}

// kces2PartsColorValues 生成 PARTS_COLOR 的精确数值映射，包含 -1 的 NONE
// kces2PartsColorValues builds the exact PARTS_COLOR numeric mapping including NONE at -1
func kces2PartsColorValues() []ValueSetValue {
	names := []string{
		"NONE", "HAIR", "EYE_BROW", "UNDER_HAIR", "ASS_HAIR", "SKIN", "HAIR_OUTLINE", "SKIN_OUTLINE", "EYE_WHITE",
		"HOKURO", "TATOO", "SOBAKASU", "MATSUGE_UP", "MATSUGE_LOW", "FUTAE", "PART_COLOR", "GRADA_COLOR", "MAKE",
		"MUGEN_COLOR", "HIGE", "SHIMI", "SHIWA", "BODY_HAIR", "MAX",
	}
	values := make([]ValueSetValue, 0, len(names))
	for index, name := range names {
		values = append(values, ValueSetValue{Name: name, Number: index - 1})
	}
	return values
}

// kces2MPNNames 记录 KCES2 1.36.0 MPN 枚举的全部声明名（下标即枚举数值）
// kces2MPNNames records every declared MPN name in KCES2 1.36.0; the slice index is the enum value
var kces2MPNNames = []string{
	"null_mpn", "Hara", "KubiScl", "UdeScl", "DouPer", "sintyou", "kata", "MuneL", "MuneS", "MuneM",
	"MuneUpDown", "MuneYori", "MuneYawaraka", "MunePosX", "MunePosY", "MuneThick", "MuneLong", "MuneDir", "DouThick1X", "DouThick1Y",
	"DouThick2X", "DouThick2Y", "DouThick3X", "DouThick3Y", "ShoulderThick", "UpperArmThickX", "UpperArmThickY", "LowerArmThickX", "LowerArmThickY", "ElbowThickX",
	"ElbowThickY", "NeckThickX", "NeckThickY", "HandSize", "DouThick4X", "DouThick4Y", "DouThick5X", "DouThick5Y", "WaistPos", "HipSize",
	"HipRot", "ThighThickX", "ThighThickY", "KneeThickX", "KneeThickY", "CalfThickX", "CalfThickY", "AnkleThickX", "AnkleThickY", "FootSize",
	"UpperArmLowerThickX", "UpperArmLowerThickY", "WristThickX", "WristThickY", "ClavicleThick", "ShoulderTension", "ThighLowerThickX", "ThighLowerThickY", "ThighShin", "HaraN",
	"ChikubiH", "ChikubiK1", "ChikubiK2", "ChikubiK2_MuneS", "ChikubiR", "ChikubiW", "Nyurin1", "Nyurin2", "Nyurin3", "Nyurin4",
	"Nyurin5", "Nyurin6", "Nyurin7", "Nyurin8", "ChikubiWearTotsu", "NyurinScale", "FatUpper", "FatUnder", "MuscleSkin", "HipYawaraka",
	"HaraYawaraka", "MuneSpringPower", "MuneSpringMove", "HaraSpringPower", "HaraSpringMove", "HipSpringPower", "HipSpringMove", "HeadX", "HeadY", "FaceShape",
	"FaceShapeSlim", "EyeSclX", "EyeSclY", "EyePosX", "EyePosY", "EyePosX_2", "EyePosY_2", "EyeClose", "EyeBallPosY", "EyeBallSclX",
	"EyeBallSclY", "EarNone", "EarElf", "EarRot", "EarScl", "NosePos", "NoseScl", "MayuShapeIn", "MayuShapeOut", "MayuX",
	"MayuY", "MayuY_2", "MayuRot", "MayuThick", "MayuLong", "Yorime", "MabutaUpIn", "MabutaUpIn2", "MabutaUpMiddle", "MabutaUpOut",
	"MabutaUpOut2", "MabutaLowIn", "MabutaLowMiddle", "MabutaLowOut", "Eyedel", "Itome", "Ha1", "Ha2", "Ha3", "Ha4",
	"Ha5", "Ha6", "FutaePosX", "FutaePosY", "FutaeRot", "HitomiHiPosX", "HitomiHiPosY", "HitomiHiSclY", "HitomiShapeUp", "HitomiShapeLow",
	"HitomiShapeIn", "HitomiShapeOutUp", "HitomiShapeOutLow", "HitomiRot", "HohoShape", "LipThick", "WearSuso", "WearMuneShadowRate", "KuikomiPants", "KuikomiStkg",
	"CheekRate", "FaceglossRate", "MayuRate", "EyeShadowRate", "EyeHiRateL", "EyeHiRateR", "LipRate", "LipTsuyaRate", "NailTsuyaRate", "SkinHiyakeRate",
	"ArmpitHairRate", "UnderHairRate", "AssHairRate", "StkgRate", "LipShadowRate", "Hanasuji", "Washibana", "EyeDel_shadowRate", "Nose_RimlightMask", "Ago_Back_Foward",
	"Ago_Long_Short", "Ago_Sharp", "AgoHaba_Large_Small", "AgoNiku_Fat_Slim", "AgoSentan_Back_Foward", "AgoSentan_Long_Short", "AgoSentan_Sharp", "AgoSentanHaba_Large_Small", "AgoSide_Back_Foward", "Cheekbone_Sharp",
	"Cheekbone_Slim_Fat", "Era_Sharp", "EyePosZ", "Face_Slim", "Face_UnderBack_Foward", "Face_UnderLarge_Small", "Ho_UnderBack_Foward", "Ho_UpperBack_Foward", "Ho_Sharp", "Ho_Down_Up",
	"Ho_Hukurami", "Hanasuji_Back_Foward", "NoseSentan_Marumi", "NoseSentan_Sharp", "Nose_Shape", "body", "moza", "head", "hairf", "hairr",
	"hairt", "hairs", "hairaho", "haircolor", "skin", "skin_nikukan", "skin_hiyake", "acctatoo", "accnail", "underhair",
	"asshair", "armpithair", "hokuro", "mayu", "lip", "lip_tsuya", "chikubi", "nyurin", "eye",
	"eye_r", "eye_hi", "eye_hi_r", "eyewhite", "eyewhite_r", "nose", "facegloss", "matsuge_up", "matsuge_low", "futae",
	"hoho_some", "eye_shadow", "cheek", "EyeDel_shadow", "nail_hi", "kuchi_naka", "sobakasu", "hige", "shiwa", "shimiibo",
	"bodyhair", "wear", "skirt", "mizugi", "mizugi_top", "mizugi_buttom", "bra", "panz", "slip", "stkg",
	"shoes", "headset", "glove", "acchead", "accha", "acchana", "accface", "acckamisub", "acckami", "accmimi",
	"accnip", "acckubi", "acckubiwa", "accheso", "accude", "accashi", "accsenaka", "accshippo", "acckoshi", "accanl",
	"accvag", "megane", "accxxx", "handitem", "acchat", "onepiece", "outerwear", "jacket", "vest", "shirt",
	"accAcc1", "accAcc2", "accAcc3", "accAcc4", "accAcc5", "accAcc6", "accAcc7", "accAcc8", "accAcc9", "accAcc10",
	"accAcc11", "accAcc12", "accAcc13", "accAcc14", "accAcc15", "accAcc16", "accAcc17", "accAcc18", "accAcc19", "accAcc20",
	"accAcc21", "accAcc22", "accAcc23", "accAcc24", "set_maidwear", "set_mywear", "set_underwear", "set_body", "set_face", "folder_eye",
	"folder_mayu", "folder_underhair", "folder_asshair", "folder_skin", "folder_eyewhite", "folder_chikubi", "folder_nyurin", "folder_matsuge_up", "folder_matsuge_low", "folder_futae",
	"folder_lip", "folder_cheek", "folder_eye_shadow", "NyurinSelect", "kousoku_upper", "kousoku_lower", "seieki_naka", "seieki_hara", "seieki_face", "seieki_mune",
	"seieki_hip", "seieki_ude", "seieki_ashi",
}
