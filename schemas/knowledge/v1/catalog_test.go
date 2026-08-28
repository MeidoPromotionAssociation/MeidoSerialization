package knowledgev1_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/schemas/editing/v1"
	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/v2/schemas/knowledge/v1"
)

func TestResolveGuidesForEveryEditingSchema(t *testing.T) {
	formats, err := editingv1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	profileFormats, err := knowledgev1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(formats, profileFormats) {
		t.Fatalf("editing schema formats and reviewed profile formats differ:\n schemas: %v\nprofiles: %v", formats, profileFormats)
	}
	for _, formatID := range formats {
		schema, found, err := editingv1.Lookup(formatID)
		if err != nil || !found {
			t.Fatalf("schema %s: found=%v err=%v", formatID, found, err)
		}
		guide, err := knowledgev1.Resolve(formatID, schema.ID, schema.JSON)
		if err != nil {
			t.Fatalf("guide %s: %v", formatID, err)
		}
		if guide.FormatID != formatID || guide.SchemaID != schema.ID || guide.SHA256 == "" || guide.FormatVerification != knowledgev1.FormatVerificationSerializationVerified || !json.Valid(guide.JSON) {
			t.Fatalf("guide %s metadata is inconsistent: %+v", formatID, guide)
		}
		var decoded knowledgev1.Guide
		if err := json.Unmarshal(guide.JSON, &decoded); err != nil {
			t.Fatal(err)
		}
		if annotation, found := firstHanAnnotation(decoded); found {
			t.Fatalf("guide %s contains a non-English Han annotation: %q", formatID, annotation)
		}
		if len(decoded.Fields) == 0 {
			t.Fatalf("guide %s has no fields", formatID)
		}
	}
}

func TestProfileCatalogReferencesPublishedSchemas(t *testing.T) {
	formats, err := knowledgev1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	if len(formats) == 0 {
		t.Fatal("format-guide profile catalog is empty")
	}
	for _, formatID := range formats {
		schema, schemaFound, err := editingv1.Lookup(formatID)
		if err != nil || !schemaFound {
			t.Fatalf("format-guide profile %s has no published schema: found=%v err=%v", formatID, schemaFound, err)
		}
		document, guideFound, err := knowledgev1.Lookup(formatID)
		if err != nil || !guideFound {
			t.Fatalf("format-guide profile %s: found=%v err=%v", formatID, guideFound, err)
		}
		if document.FormatID != formatID || document.Version == "" || document.ID == "" || document.SHA256 == "" || !json.Valid(document.JSON) {
			t.Fatalf("format-guide profile %s metadata = %+v", formatID, document)
		}
		var profile knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &profile); err != nil {
			t.Fatal(err)
		}
		if profile.SchemaID != schema.ID || profile.SchemaURI != "meido://schemas/"+formatID || profile.FormatVerification.Level != knowledgev1.FormatVerificationSerializationVerified || profile.FormatVerification.Authority != knowledgev1.ReviewAuthorityAI || profile.FieldCoverage.Total == 0 || len(profile.Sources) == 0 || len(profile.Fields) == 0 {
			t.Fatalf("format-guide profile %s contract = %+v", formatID, profile)
		}
		for _, evidence := range profile.Sources {
			if evidence.Path == "" || evidence.LineStart < 1 || evidence.LineEnd < evidence.LineStart {
				t.Fatalf("format-guide profile %s has an imprecise source reference: %+v", formatID, evidence)
			}
		}
		if _, err := knowledgev1.Resolve(formatID, schema.ID, schema.JSON); err != nil {
			t.Fatalf("format-guide profile %s has an invalid Schema pointer or field path: %v", formatID, err)
		}
	}
}

func TestProfileEvidenceKindsAndVerification(t *testing.T) {
	formats, err := knowledgev1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	for _, formatID := range formats {
		document, found, err := knowledgev1.Lookup(formatID)
		if err != nil || !found {
			t.Fatalf("profile %s: found=%v err=%v", formatID, found, err)
		}
		var profile knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &profile); err != nil {
			t.Fatal(err)
		}

		auditSources := func(owner string, sources []knowledgev1.Source) {
			t.Helper()
			for _, evidence := range sources {
				path := strings.ReplaceAll(evidence.Path, `\`, "/")
				switch evidence.Kind {
				case knowledgev1.SourceKindGame:
					if strings.HasPrefix(path, "game/") {
						t.Fatalf("profile %s %s game source path includes the game/ prefix: %+v", formatID, owner, evidence)
					}
				case knowledgev1.SourceKindImplementation:
					if strings.HasPrefix(path, "game/") {
						t.Fatalf("profile %s %s implementation source path includes the game/ prefix: %+v", formatID, owner, evidence)
					}
				case knowledgev1.SourceKindRuntimeObservation:
				default:
					t.Fatalf("profile %s %s has invalid source kind %q: %+v", formatID, owner, evidence.Kind, evidence)
				}
			}
		}
		requireEvidence := func(owner string, verification knowledgev1.FieldVerification, sources []knowledgev1.Source) {
			t.Helper()
			hasKind := func(kind string) bool {
				for _, evidence := range sources {
					if evidence.Kind == kind {
						return true
					}
				}
				return false
			}
			if verification.SourceSemantics != nil && (verification.Serialization == nil || !hasKind(knowledgev1.SourceKindGame)) {
				t.Fatalf("profile %s source-verified %s lacks serialization or game-source evidence", formatID, owner)
			}
			if verification.GameBehavior != nil && !hasKind(knowledgev1.SourceKindRuntimeObservation) {
				t.Fatalf("profile %s game-behavior-verified %s has no runtime observation", formatID, owner)
			}
		}

		auditSources("guide", profile.Sources)
		for _, field := range profile.Fields {
			owner := "field " + field.JSONPath
			auditSources(owner, field.Evidence)
			requireEvidence(owner, field.Verification, field.Evidence)
		}
		for _, pattern := range profile.FieldPatterns {
			owner := "field pattern " + pattern.JSONPathPattern
			auditSources(owner, pattern.Evidence)
			requireEvidence(owner, pattern.Verification, pattern.Evidence)
		}
		for _, valueSet := range profile.ValueSets {
			owner := "value set " + valueSet.ID
			auditSources(owner, valueSet.Evidence)
			requireEvidence(owner, valueSet.Verification, valueSet.Evidence)
		}
		for _, rule := range profile.Rules {
			auditSources("rule "+rule.ID, rule.Evidence)
		}
		for _, command := range profile.Commands {
			auditSources("command "+command.Name, command.Evidence)
		}
	}
}

func TestAIReviewedProfilesRecordExplicitAuthority(t *testing.T) {
	if knowledgev1.IsHumanReviewAuthority(knowledgev1.ReviewAuthorityAI) || !knowledgev1.IsHumanReviewAuthority(knowledgev1.ReviewAuthorityHuman) {
		t.Fatal("review authority constants are inconsistent")
	}
	formats, err := knowledgev1.Formats()
	if err != nil {
		t.Fatal(err)
	}
	for _, formatID := range formats {
		document, found, err := knowledgev1.Lookup(formatID)
		if err != nil || !found {
			t.Fatalf("profile %s: found=%v err=%v", formatID, found, err)
		}
		var profile knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &profile); err != nil {
			t.Fatal(err)
		}
		if profile.FormatVerification.Authority != knowledgev1.ReviewAuthorityAI {
			t.Fatalf("AI-reviewed profile %s has authority %q", formatID, profile.FormatVerification.Authority)
		}
		requireAI := func(owner string, verification knowledgev1.FieldVerification) {
			t.Helper()
			for _, claim := range []*knowledgev1.VerificationClaim{verification.Serialization, verification.SourceSemantics, verification.GameBehavior} {
				if claim != nil && claim.Authority != knowledgev1.ReviewAuthorityAI {
					t.Fatalf("AI-reviewed profile %s %s has authority %q", formatID, owner, claim.Authority)
				}
			}
		}
		for _, field := range profile.Fields {
			requireAI("field "+field.JSONPath, field.Verification)
		}
		for _, fieldPattern := range profile.FieldPatterns {
			requireAI("pattern "+fieldPattern.JSONPathPattern, fieldPattern.Verification)
		}
		for _, valueSet := range profile.ValueSets {
			requireAI("value set "+valueSet.ID, valueSet.Verification)
		}
	}
}

func TestRepresentativeReviewedFields(t *testing.T) {
	tests := []struct {
		formatID     string
		path         string
		verification string
	}{
		{formatID: "com3d2.menu", path: "/Commands", verification: "source_semantics"},
		{formatID: "kces.dbconf", path: "/version", verification: "source_semantics"},
		{formatID: "kces.dbcol", path: "/colliders", verification: "source_semantics"},
		{formatID: "kces.bytes", path: "/dataBase64", verification: "serialization"},
	}
	for _, test := range tests {
		schema, found, err := editingv1.Lookup(test.formatID)
		if err != nil || !found {
			t.Fatalf("schema %s: found=%v err=%v", test.formatID, found, err)
		}
		document, err := knowledgev1.Resolve(test.formatID, schema.ID, schema.JSON)
		if err != nil {
			t.Fatal(err)
		}
		if document.FormatVerification != knowledgev1.FormatVerificationSerializationVerified {
			t.Fatalf("guide %s format verification = %q", test.formatID, document.FormatVerification)
		}
		var guide knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &guide); err != nil {
			t.Fatal(err)
		}
		foundField := false
		for _, field := range guide.Fields {
			if field.JSONPath == test.path {
				foundField = hasVerification(field.Verification, test.verification) && field.GameUsage != "" && len(field.Evidence) != 0
				break
			}
		}
		if !foundField {
			t.Fatalf("guide %s has no %s-verified field %s", test.formatID, test.verification, test.path)
		}
	}
}

func TestRepresentativeReviewedFieldPatterns(t *testing.T) {
	tests := []struct {
		formatID     string
		path         string
		verification string
	}{
		{formatID: "com3d2.menu", path: "/Commands/*/Command", verification: "source_semantics"},
		{formatID: "kces.dbconf", path: "/{version,damping,elasticity,stiffness,inert,radius}", verification: "source_semantics"},
		{formatID: "kces.dbcol", path: "/colliders/*/{type,collider}", verification: "serialization"},
		{formatID: "kces.dsbconf", path: "/{radius,mass,gravity,drag,maxVelocity,worldMoveInfluence,worldRotationInfluence,clampPositionLength,clampRotationAngle,structDistanceStiffness,bendDistanceStiffness,nearDistanceLength,nearDistanceStiffness,restoreRotation,triangleBend,volumeStretchStiffness,volumeShearStiffness,penetrationConnectDistance,penetrationDistance,penetrationRadius,springDirectionAtten,springDistanceAtten}", verification: "serialization"},
	}
	for _, test := range tests {
		document, found, err := knowledgev1.Lookup(test.formatID)
		if err != nil || !found {
			t.Fatalf("profile %s: found=%v err=%v", test.formatID, found, err)
		}
		var guide knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &guide); err != nil {
			t.Fatal(err)
		}
		foundPattern := false
		for _, fieldPattern := range guide.FieldPatterns {
			if fieldPattern.JSONPathPattern == test.path {
				foundPattern = hasVerification(fieldPattern.Verification, test.verification) && fieldPattern.GameUsage != "" && len(fieldPattern.Evidence) != 0
				break
			}
		}
		if !foundPattern {
			t.Fatalf("guide %s has no %s-verified field pattern %s", test.formatID, test.verification, test.path)
		}
	}
}

func TestEffectiveGuideAppliesWholeFileSerializationVerification(t *testing.T) {
	schema, found, err := editingv1.Lookup("com3d2.mate")
	if err != nil || !found {
		t.Fatalf("schema: found=%v err=%v", found, err)
	}
	document, err := knowledgev1.Resolve("com3d2.mate", schema.ID, schema.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if document.FormatVerification != knowledgev1.FormatVerificationSerializationVerified {
		t.Fatalf("format verification = %q", document.FormatVerification)
	}
	var guide knowledgev1.Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		t.Fatal(err)
	}
	for _, field := range guide.Fields {
		if field.Verification.Serialization == nil {
			t.Fatalf("field %s lacks whole-file serialization verification", field.JSONPath)
		}
	}
	if guide.FieldCoverage.SerializationVerified != guide.FieldCoverage.Total || guide.FieldCoverage.SchemaDerived != 0 {
		t.Fatalf("field coverage = %+v", guide.FieldCoverage)
	}
}

func TestCOM3D2MenuCommandReferenceIsCompleteAndStructured(t *testing.T) {
	document, found, err := knowledgev1.Lookup("com3d2.menu")
	if err != nil || !found {
		t.Fatalf("com3d2.menu guide: found=%v err=%v", found, err)
	}
	var guide knowledgev1.Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		t.Fatal(err)
	}

	expectedOpcodes := []string{
		"end", "#if", "#else", "#endif", "name", "setumei", "category", "icon", "icons", "iconl", "setstr", "catno", "unsetitem", "priority", "メニューフォルダ", "collabo", "color_set", "saveitem", "set", "setname", "ver",
		"crc_exp", "crc_def_exp", "crc_target_slotno", "crc_part_hide_slot", "crc_part_hide_move", "crc_mayu_front", "crc_mayu_alpha", "crc_hair_length", "crc_mat_alpha", "crc_target_body_type", "parthidemove", "skirt_phys",
		"アイテム", "alldelmenu", "アイテム条件", "if", "setprop", "アイテムパラメータ", "半脱ぎ", "リソース参照", "setslotitem", "additem", "nofloory", "maskitem", "delitem",
		"node消去", "node表示", "パーツnode消去", "パーツnode表示", "mask消去", "cutout消去", "cutout消去cc", "color", "mancolor", "tex", "テクスチャ変更", "prop", "テクスチャ乗算", "テクスチャ合成", "テクスチャセット合成", "マテリアル変更", "shader",
		"アタッチポイントの設定", "blendset", "paramset", "commenttype", "useredit", "bonemorph", "length", "anime", "param2", "animematerial", "meshmorph", "addbonemorph", "乳首", "ちんこ", "toelock",
	}
	opcodes := make(map[string]knowledgev1.Command, len(expectedOpcodes))
	for _, command := range guide.Commands {
		for _, opcode := range append([]string{command.Name}, command.Aliases...) {
			if _, duplicate := opcodes[opcode]; duplicate {
				t.Fatalf("duplicate com3d2.menu opcode or alias %q", opcode)
			}
			opcodes[opcode] = command
		}
		if len(command.Contexts) == 0 || len(command.Forms) == 0 || command.GameEffect == "" || command.EditGuidance == "" || command.Risk == "" || len(command.Evidence) == 0 {
			t.Fatalf("incomplete command contract: %+v", command)
		}
		for _, form := range command.Forms {
			if form.Syntax == "" || len(form.ReviewedIn) == 0 {
				t.Fatalf("incomplete form for %s: %+v", command.Name, form)
			}
			for _, argument := range form.Arguments {
				if argument.Position < 1 || argument.Name == "" || argument.Type == "" || argument.Description == "" {
					t.Fatalf("incomplete argument for %s: %+v", command.Name, argument)
				}
			}
		}
	}
	for _, opcode := range expectedOpcodes {
		if _, exists := opcodes[opcode]; !exists {
			t.Errorf("com3d2.menu command reference is missing opcode %q", opcode)
		}
	}
	if len(opcodes) != len(expectedOpcodes) {
		t.Fatalf("com3d2.menu opcode count=%d, expected=%d", len(opcodes), len(expectedOpcodes))
	}
	for commandName, minimumForms := range map[string]int{"ver": 2, "アイテム条件": 3, "if": 2, "additem": 6, "bonemorph": 2} {
		if len(opcodes[commandName].Forms) < minimumForms {
			t.Errorf("command %s has %d forms, expected at least %d", commandName, len(opcodes[commandName].Forms), minimumForms)
		}
	}
}

func TestCOM3D2MenuValueSetsResolveEnumArguments(t *testing.T) {
	document, found, err := knowledgev1.Lookup("com3d2.menu")
	if err != nil || !found {
		t.Fatalf("com3d2.menu guide: found=%v err=%v", found, err)
	}
	var guide knowledgev1.Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		t.Fatal(err)
	}
	valueSets := make(map[string]knowledgev1.ValueSet, len(guide.ValueSets))
	for _, valueSet := range guide.ValueSets {
		if _, duplicate := valueSets[valueSet.ID]; duplicate {
			t.Fatalf("duplicate value set %q", valueSet.ID)
		}
		if valueSet.CSharpType == "" || valueSet.Description == "" || valueSet.EditGuidance == "" || valueSet.Verification.SourceSemantics == nil || valueSet.Verification.Serialization == nil || len(valueSet.ReviewedIn) == 0 || len(valueSet.Values) == 0 || len(valueSet.Evidence) == 0 {
			t.Fatalf("incomplete value set: %+v", valueSet)
		}
		valueNames := make(map[string]struct{}, len(valueSet.Values))
		valueNumbers := make(map[int]struct{}, len(valueSet.Values))
		for _, value := range valueSet.Values {
			if value.Name == "" {
				t.Fatalf("empty value name in %s", valueSet.ID)
			}
			if _, duplicate := valueNames[value.Name]; duplicate {
				t.Fatalf("duplicate name %q in %s", value.Name, valueSet.ID)
			}
			if _, duplicate := valueNumbers[value.Number]; duplicate {
				t.Fatalf("duplicate number %d in %s", value.Number, valueSet.ID)
			}
			valueNames[value.Name] = struct{}{}
			valueNumbers[value.Number] = struct{}{}
		}
		valueSets[valueSet.ID] = valueSet
	}
	for valueSetID, expectedCount := range map[string]int{
		"com3d2.mpn.2_48":           133,
		"com3d2.mpn.3_48":           234,
		"com3d2.tbody_slot_id.2_48": 59,
		"com3d2.tbody_slot_id.3_48": 74,
	} {
		if got := len(valueSets[valueSetID].Values); got != expectedCount {
			t.Errorf("value set %s count=%d, expected=%d", valueSetID, got, expectedCount)
		}
	}
	for _, command := range guide.Commands {
		for _, form := range command.Forms {
			for _, argument := range form.Arguments {
				for _, ref := range argument.ValueSetRefs {
					if _, exists := valueSets[ref]; !exists {
						t.Errorf("command %s argument %s references missing value set %s", command.Name, argument.Name, ref)
					}
				}
				if (strings.Contains(argument.Type, "MPN") || strings.Contains(argument.Type, "SlotID") || strings.Contains(argument.Type, "PARTS_COLOR") || strings.Contains(argument.Type, "SystemMaterial")) && len(argument.ValueSetRefs) == 0 {
					t.Errorf("command %s argument %s has enum-like type %q without value-set refs", command.Name, argument.Name, argument.Type)
				}
			}
		}
	}
}

func hasVerification(verification knowledgev1.FieldVerification, kind string) bool {
	switch kind {
	case "serialization":
		return verification.Serialization != nil
	case "source_semantics":
		return verification.SourceSemantics != nil
	case "game_behavior":
		return verification.GameBehavior != nil
	default:
		return false
	}
}

func firstHanAnnotation(guide knowledgev1.Guide) (string, bool) {
	annotations := []string{guide.Title, guide.Summary, guide.FormatVerification.Level, guide.FormatVerification.Authority, guide.FormatVerification.Notes}
	for _, source := range guide.Sources {
		annotations = append(annotations, source.Observation)
	}
	for _, field := range guide.Fields {
		annotations = append(annotations, field.Title, field.Description, field.GameUsage, field.EditRole, field.EditGuidance, field.Risk)
		annotations = append(annotations, field.Constraints...)
		for _, enumValue := range field.EnumValues {
			annotations = append(annotations, enumValue.Meaning)
		}
		for _, source := range field.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	for _, pattern := range guide.FieldPatterns {
		annotations = append(annotations, pattern.Title, pattern.Description, pattern.GameUsage, pattern.EditRole, pattern.EditGuidance)
		annotations = append(annotations, pattern.Constraints...)
		for _, source := range pattern.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	for _, rule := range guide.Rules {
		annotations = append(annotations, rule.Severity, rule.Summary, rule.Details)
		for _, source := range rule.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	for _, command := range guide.Commands {
		annotations = append(annotations, command.Contexts...)
		annotations = append(annotations, command.GameEffect, command.EditGuidance, command.Risk)
		for _, form := range command.Forms {
			annotations = append(annotations, form.ReviewedIn...)
			annotations = append(annotations, form.Notes)
			for _, argument := range form.Arguments {
				annotations = append(annotations, argument.Name, argument.Type, argument.Description)
			}
		}
		for _, source := range command.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	for _, valueSet := range guide.ValueSets {
		annotations = append(annotations, valueSet.ID, valueSet.CSharpType, valueSet.Description, valueSet.EditGuidance)
		annotations = append(annotations, valueSet.ReviewedIn...)
		for _, source := range valueSet.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	annotations = append(annotations, guide.Invariants...)
	annotations = append(annotations, guide.Workflow...)
	annotations = append(annotations, guide.Warnings...)
	for _, annotation := range annotations {
		if containsHan(annotation) {
			return annotation, true
		}
	}
	return "", false
}

func containsHan(text string) bool {
	for _, value := range text {
		if unicode.Is(unicode.Han, value) {
			return true
		}
	}
	return false
}
