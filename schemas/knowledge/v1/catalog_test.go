package knowledgev1_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/editing/v1"
	knowledgev1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/knowledge/v1"
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
		if guide.FormatID != formatID || guide.SchemaID != schema.ID || guide.SHA256 == "" || guide.Coverage == knowledgev1.CoverageSchemaOnly || !json.Valid(guide.JSON) {
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
		if profile.SchemaID != schema.ID || profile.SchemaURI != "meido://schemas/"+formatID || profile.Coverage.Level == "" || profile.Coverage.ReviewedFields <= 0 || len(profile.Sources) == 0 || len(profile.Fields) == 0 {
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

func TestProfileEvidenceKindsAndConfidence(t *testing.T) {
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
					if !strings.HasPrefix(path, "game/") {
						t.Fatalf("profile %s %s game source is outside game/: %+v", formatID, owner, evidence)
					}
				case knowledgev1.SourceKindImplementation:
					if strings.HasPrefix(path, "game/") {
						t.Fatalf("profile %s %s implementation source masquerades as game source: %+v", formatID, owner, evidence)
					}
				default:
					t.Fatalf("profile %s %s has invalid source kind %q: %+v", formatID, owner, evidence.Kind, evidence)
				}
			}
		}
		requireGameEvidence := func(owner, confidence string, sources []knowledgev1.Source) {
			t.Helper()
			if confidence != knowledgev1.ConfidenceVerified && confidence != knowledgev1.ConfidenceHumanVerified {
				return
			}
			for _, evidence := range sources {
				if evidence.Kind == knowledgev1.SourceKindGame {
					return
				}
			}
			t.Fatalf("profile %s verified %s has no game source", formatID, owner)
		}

		auditSources("guide", profile.Sources)
		for _, field := range profile.Fields {
			owner := "field " + field.JSONPath
			auditSources(owner, field.Evidence)
			requireGameEvidence(owner, field.Confidence, field.Evidence)
		}
		for _, pattern := range profile.FieldPatterns {
			owner := "field pattern " + pattern.JSONPathPattern
			auditSources(owner, pattern.Evidence)
			requireGameEvidence(owner, pattern.Confidence, pattern.Evidence)
		}
		for _, valueSet := range profile.ValueSets {
			owner := "value set " + valueSet.ID
			auditSources(owner, valueSet.Evidence)
			requireGameEvidence(owner, valueSet.Confidence, valueSet.Evidence)
		}
		for _, rule := range profile.Rules {
			auditSources("rule "+rule.ID, rule.Evidence)
		}
		for _, command := range profile.Commands {
			auditSources("command "+command.Name, command.Evidence)
		}
	}
}

func TestAIReviewedProfilesDoNotClaimReservedHumanPrefix(t *testing.T) {
	if knowledgev1.IsHumanReviewed(knowledgev1.CoverageRuntimeVerified) ||
		!knowledgev1.IsHumanReviewed(knowledgev1.CoverageHumanRuntimeVerified) ||
		!knowledgev1.IsHumanReviewed(knowledgev1.CoverageHumanSerializationVerified) ||
		!knowledgev1.IsHumanReviewed(knowledgev1.ConfidenceHumanVerified) {
		t.Fatal("human review prefix constants do not preserve review authority")
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
		if knowledgev1.IsHumanReviewed(profile.Coverage.Level) {
			t.Fatalf("AI-reviewed profile %s claims human coverage %q", formatID, profile.Coverage.Level)
		}
		for _, field := range profile.Fields {
			if knowledgev1.IsHumanReviewed(field.Confidence) {
				t.Fatalf("AI-reviewed profile %s field %s claims human confidence %q", formatID, field.JSONPath, field.Confidence)
			}
		}
		for _, fieldPattern := range profile.FieldPatterns {
			if knowledgev1.IsHumanReviewed(fieldPattern.Confidence) {
				t.Fatalf("AI-reviewed profile %s pattern %s claims human confidence %q", formatID, fieldPattern.JSONPathPattern, fieldPattern.Confidence)
			}
		}
		for _, valueSet := range profile.ValueSets {
			if knowledgev1.IsHumanReviewed(valueSet.Confidence) {
				t.Fatalf("AI-reviewed profile %s value set %s claims human confidence %q", formatID, valueSet.ID, valueSet.Confidence)
			}
		}
	}
}

func TestRepresentativeReviewedFields(t *testing.T) {
	tests := []struct {
		formatID   string
		path       string
		confidence string
		coverage   string
	}{
		{formatID: "com3d2.menu", path: "/Commands", confidence: knowledgev1.ConfidenceVerified, coverage: knowledgev1.CoverageRuntimeVerified},
		{formatID: "kces.dbconf", path: "/dynamicBoneStatus", confidence: knowledgev1.ConfidenceVerified, coverage: knowledgev1.CoverageRuntimeVerified},
		{formatID: "kces.dbcol", path: "/colliderPackage", confidence: knowledgev1.ConfidenceVerified, coverage: knowledgev1.CoverageRuntimeVerified},
		{formatID: "kces.bytes", path: "/dataBase64", confidence: knowledgev1.ConfidenceSerializationOnly, coverage: knowledgev1.CoverageSerializationVerified},
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
		if document.Coverage != test.coverage {
			t.Fatalf("guide %s coverage = %q", test.formatID, document.Coverage)
		}
		var guide knowledgev1.Guide
		if err := json.Unmarshal(document.JSON, &guide); err != nil {
			t.Fatal(err)
		}
		foundField := false
		for _, field := range guide.Fields {
			if field.JSONPath == test.path {
				foundField = field.Confidence == test.confidence && field.GameUsage != "" && len(field.Evidence) != 0
				break
			}
		}
		if !foundField {
			t.Fatalf("guide %s has no %s field %s", test.formatID, test.confidence, test.path)
		}
	}
}

func TestRepresentativeReviewedFieldPatterns(t *testing.T) {
	tests := []struct {
		formatID   string
		path       string
		confidence string
	}{
		{formatID: "com3d2.menu", path: "/Commands/*/Command", confidence: knowledgev1.ConfidenceVerified},
		{formatID: "kces.dbconf", path: "/dynamicBoneStatus/{version,damping,elasticity,stiffness,inert,radius}", confidence: knowledgev1.ConfidenceVerified},
		{formatID: "kces.dbcol", path: "/colliderPackage/colliders/*/{type,collider,colliderRaw}", confidence: knowledgev1.ConfidenceSerializationOnly},
		{formatID: "kces.dsbconf", path: "/clothParams/{radius,mass,gravity,drag,maxVelocity,worldMoveInfluence,worldRotationInfluence,clampPositionLength,clampRotationAngle,structDistanceStiffness,bendDistanceStiffness,nearDistanceLength,nearDistanceStiffness,restoreRotation,triangleBend,volumeStretchStiffness,volumeShearStiffness,penetrationConnectDistance,penetrationDistance,penetrationRadius,springDirectionAtten,springDistanceAtten}", confidence: knowledgev1.ConfidenceSerializationOnly},
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
				foundPattern = fieldPattern.Confidence == test.confidence && fieldPattern.GameUsage != "" && len(fieldPattern.Evidence) != 0
				break
			}
		}
		if !foundPattern {
			t.Fatalf("guide %s has no %s field pattern %s", test.formatID, test.confidence, test.path)
		}
	}
}

func TestEffectiveGuideKeepsUnreviewedFieldsSchemaOnly(t *testing.T) {
	schema, found, err := editingv1.Lookup("com3d2.mate")
	if err != nil || !found {
		t.Fatalf("schema: found=%v err=%v", found, err)
	}
	document, err := knowledgev1.Resolve("com3d2.mate", schema.ID, schema.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if document.Coverage != knowledgev1.CoverageRuntimeVerified {
		t.Fatalf("coverage = %q", document.Coverage)
	}
	var guide knowledgev1.Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		t.Fatal(err)
	}
	for _, field := range guide.Fields {
		if field.Confidence == knowledgev1.ConfidenceSchemaOnly {
			return
		}
	}
	t.Fatal("runtime-reviewed guide has no schema_only fields to preserve")
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
		if valueSet.CSharpType == "" || valueSet.Description == "" || valueSet.EditGuidance == "" || valueSet.Confidence != knowledgev1.ConfidenceVerified || len(valueSet.ReviewedIn) == 0 || len(valueSet.Values) == 0 || len(valueSet.Evidence) == 0 {
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

func firstHanAnnotation(guide knowledgev1.Guide) (string, bool) {
	annotations := []string{guide.Title, guide.Summary, guide.Coverage.Level, guide.Coverage.Notes}
	for _, source := range guide.Sources {
		annotations = append(annotations, source.Observation)
	}
	for _, field := range guide.Fields {
		annotations = append(annotations, field.Title, field.Description, field.GameUsage, field.EditRole, field.EditGuidance, field.Risk, field.Confidence)
		annotations = append(annotations, field.Constraints...)
		for _, enumValue := range field.EnumValues {
			annotations = append(annotations, enumValue.Meaning)
		}
		for _, source := range field.Evidence {
			annotations = append(annotations, source.Observation)
		}
	}
	for _, pattern := range guide.FieldPatterns {
		annotations = append(annotations, pattern.Title, pattern.Description, pattern.GameUsage, pattern.EditRole, pattern.EditGuidance, pattern.Confidence)
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
		annotations = append(annotations, valueSet.ID, valueSet.CSharpType, valueSet.Description, valueSet.EditGuidance, valueSet.Confidence)
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
