package knowledgev1

import (
	"sort"
	"strings"
)

// The profile catalog contains the source-reviewed portion of each effective
// guide. Resolve merges these fields with the complete schema-derived field
// inventory, so an unreviewed field remains explicitly schema_only.
var reviewedProfiles = buildReviewedProfiles()

func buildReviewedProfiles() map[string]Guide {
	profiles := make(map[string]Guide)
	registerProfiles(profiles, com3d2Profiles())
	registerProfiles(profiles, kcesProfiles())
	registerProfiles(profiles, kcesContainerProfiles())
	registerProfiles(profiles, kcesPayloadProfiles())
	return profiles
}

func registerProfiles(destination, source map[string]Guide) {
	for formatID, guide := range source {
		if _, exists := destination[formatID]; exists {
			panic("duplicate format-guide profile " + formatID)
		}
		destination[formatID] = finalizeProfile(formatID, guide)
	}
}

func finalizeProfile(formatID string, guide Guide) Guide {
	guide.ID = IDPrefix + formatID
	guide.FormatID = formatID
	guide.Version = Version
	guide.SchemaURI = "meido://schemas/" + formatID
	guide.SchemaID = "urn:meido-serialization:editing-json:v1:" + formatID
	if guide.Coverage.Level == "" {
		guide.Coverage.Level = CoverageRuntimeVerified
	}
	if !isKnownCoverageLevel(guide.Coverage.Level) {
		panic("invalid format-guide coverage " + guide.Coverage.Level + " for " + formatID)
	}
	if guide.Coverage.Notes == "" {
		guide.Coverage.Notes = "The cited game reader or writer was reviewed against source by AI. Only fields explicitly marked verified have reviewed runtime semantics; all merged schema_only fields remain opaque. The human_ prefix is reserved for explicit human approval."
	}
	validateSources(formatID, "guide", guide.Sources)
	reviewedFields := 0
	for index := range guide.Fields {
		field := &guide.Fields[index]
		if field.SchemaPointer == "" && strings.Count(field.JSONPath, "/") == 1 {
			field.SchemaPointer = "#/properties/" + escapeJSONPointer(strings.TrimPrefix(field.JSONPath, "/"))
		}
		if field.Confidence == "" {
			field.Confidence = confidenceForEvidence(field.Evidence)
		}
		if !isKnownConfidence(field.Confidence) {
			panic("invalid field confidence " + field.Confidence + " for " + formatID + field.JSONPath)
		}
		validateSources(formatID, "field "+field.JSONPath, field.Evidence)
		requireGameEvidence(formatID, "field "+field.JSONPath, field.Confidence, field.Evidence)
		if field.Risk == "" {
			field.Risk = "medium"
		}
		if field.Confidence != ConfidenceSchemaOnly {
			reviewedFields++
		}
	}
	for index := range guide.FieldPatterns {
		fieldPattern := &guide.FieldPatterns[index]
		if fieldPattern.Confidence == "" {
			fieldPattern.Confidence = confidenceForEvidence(fieldPattern.Evidence)
		}
		if !isKnownConfidence(fieldPattern.Confidence) {
			panic("invalid field-pattern confidence " + fieldPattern.Confidence + " for " + formatID + fieldPattern.JSONPathPattern)
		}
		validateSources(formatID, "field pattern "+fieldPattern.JSONPathPattern, fieldPattern.Evidence)
		requireGameEvidence(formatID, "field pattern "+fieldPattern.JSONPathPattern, fieldPattern.Confidence, fieldPattern.Evidence)
	}
	guide.Coverage.ReviewedFields = reviewedFields
	finalizeValueSets(formatID, guide.ValueSets)
	validateCommandValueSetRefs(formatID, guide.Commands, guide.ValueSets)
	for _, rule := range guide.Rules {
		validateSources(formatID, "rule "+rule.ID, rule.Evidence)
	}
	if len(guide.Rules) == 0 {
		guide.Rules = []Rule{preserveUnknownRule()}
	} else {
		guide.Rules = append(guide.Rules, preserveUnknownRule())
	}
	if len(guide.Workflow) == 0 {
		guide.Workflow = standardWorkflow(formatID)
	}
	if len(guide.Warnings) == 0 {
		guide.Warnings = standardWarnings()
	}
	return guide
}

func finalizeValueSets(formatID string, valueSets []ValueSet) {
	ids := make(map[string]struct{}, len(valueSets))
	for index := range valueSets {
		valueSet := &valueSets[index]
		if valueSet.ID == "" || valueSet.CSharpType == "" || valueSet.Description == "" || valueSet.EditGuidance == "" || len(valueSet.ReviewedIn) == 0 || len(valueSet.Values) == 0 {
			panic("incomplete value set for " + formatID)
		}
		if _, exists := ids[valueSet.ID]; exists {
			panic("duplicate value set " + valueSet.ID + " for " + formatID)
		}
		ids[valueSet.ID] = struct{}{}
		if valueSet.Confidence == "" {
			valueSet.Confidence = confidenceForEvidence(valueSet.Evidence)
		}
		if !isKnownConfidence(valueSet.Confidence) {
			panic("invalid value-set confidence " + valueSet.Confidence + " for " + valueSet.ID)
		}
		validateSources(formatID, "value set "+valueSet.ID, valueSet.Evidence)
		requireGameEvidence(formatID, "value set "+valueSet.ID, valueSet.Confidence, valueSet.Evidence)
		for _, value := range valueSet.Values {
			if value.Name == "" {
				panic("unnamed value in value set " + valueSet.ID + " for " + formatID)
			}
		}
	}
}

func validateCommandValueSetRefs(formatID string, commands []Command, valueSets []ValueSet) {
	knownIDs := make(map[string]struct{}, len(valueSets))
	for _, valueSet := range valueSets {
		knownIDs[valueSet.ID] = struct{}{}
	}
	for _, command := range commands {
		if command.Name == "" || len(command.Contexts) == 0 || len(command.Forms) == 0 || command.GameEffect == "" || command.EditGuidance == "" || command.Risk == "" || len(command.Evidence) == 0 {
			panic("incomplete command " + command.Name + " for " + formatID)
		}
		validateSources(formatID, "command "+command.Name, command.Evidence)
		for _, form := range command.Forms {
			if form.Syntax == "" || len(form.ReviewedIn) == 0 {
				panic("incomplete command form for " + command.Name + " in " + formatID)
			}
			for _, argument := range form.Arguments {
				if argument.Position < 1 || argument.Name == "" || argument.Type == "" || argument.Description == "" {
					panic("incomplete command argument for " + command.Name + " in " + formatID)
				}
				for _, valueSetRef := range argument.ValueSetRefs {
					if _, known := knownIDs[valueSetRef]; !known {
						panic("unknown command value set " + valueSetRef + " for " + command.Name + " in " + formatID)
					}
				}
			}
		}
	}
}

func confidenceForEvidence(evidence []Source) string {
	hasImplementationSource := false
	for _, item := range evidence {
		switch item.Kind {
		case SourceKindGame:
			return ConfidenceVerified
		case SourceKindImplementation:
			hasImplementationSource = true
		}
	}
	if hasImplementationSource {
		return ConfidenceSerializationOnly
	}
	return ConfidenceSchemaOnly
}

func requireGameEvidence(formatID, owner, confidence string, evidence []Source) {
	if confidence != ConfidenceVerified && confidence != ConfidenceHumanVerified {
		return
	}
	for _, item := range evidence {
		if item.Kind == SourceKindGame {
			return
		}
	}
	panic("verified " + owner + " has no game source for " + formatID)
}

func validateSources(formatID, owner string, sources []Source) {
	for _, item := range sources {
		if item.Kind != SourceKindGame && item.Kind != SourceKindImplementation {
			panic("invalid source kind " + item.Kind + " for " + owner + " in " + formatID)
		}
		path := strings.ReplaceAll(item.Path, "\\", "/")
		if item.Path == "" || item.LineStart < 1 || item.LineEnd < item.LineStart {
			panic("imprecise source for " + owner + " in " + formatID)
		}
		if item.Kind == SourceKindGame && !strings.HasPrefix(path, "game/") {
			panic("game source outside game/ for " + owner + " in " + formatID)
		}
		if item.Kind == SourceKindImplementation && strings.HasPrefix(path, "game/") {
			panic("implementation source inside game/ for " + owner + " in " + formatID)
		}
	}
}

func isKnownCoverageLevel(level string) bool {
	switch level {
	case CoverageRuntimeVerified, CoverageSerializationVerified, CoverageSchemaOnly,
		CoverageHumanRuntimeVerified, CoverageHumanSerializationVerified:
		return true
	default:
		return false
	}
}

func isKnownConfidence(confidence string) bool {
	switch confidence {
	case ConfidenceVerified, ConfidenceSerializationOnly, ConfidenceSchemaOnly,
		ConfidenceHumanVerified, ConfidenceHumanSerializationOnly:
		return true
	default:
		return false
	}
}

func profileGuide(formatID string) (Guide, bool) {
	guide, found := reviewedProfiles[formatID]
	return guide, found
}

func profileFormats() []string {
	formats := make([]string, 0, len(reviewedProfiles))
	for formatID := range reviewedProfiles {
		formats = append(formats, formatID)
	}
	sort.Strings(formats)
	return formats
}

func source(gameVersion, path, symbol string, lineStart, lineEnd int, observation string) Source {
	return Source{
		Kind:        SourceKindGame,
		GameVersion: gameVersion,
		Path:        path,
		Symbol:      symbol,
		LineStart:   lineStart,
		LineEnd:     lineEnd,
		Observation: observation,
	}
}

func implementationSource(gameVersion, path, symbol string, lineStart, lineEnd int, observation string) Source {
	return Source{
		Kind:        SourceKindImplementation,
		GameVersion: gameVersion,
		Path:        path,
		Symbol:      symbol,
		LineStart:   lineStart,
		LineEnd:     lineEnd,
		Observation: observation,
	}
}

func field(path, title, description, gameUsage, editRole, editGuidance, risk string, evidence ...Source) Field {
	return Field{
		JSONPath:     path,
		Title:        title,
		Description:  description,
		GameUsage:    gameUsage,
		EditRole:     editRole,
		EditGuidance: editGuidance,
		Risk:         risk,
		Evidence:     evidence,
		Confidence:   confidenceForEvidence(evidence),
	}
}

func fieldFrom(evidence ...Source) func(string, string, string, string, string, string, string) Field {
	return func(path, title, description, gameUsage, editRole, editGuidance, risk string) Field {
		return field(path, title, description, gameUsage, editRole, editGuidance, risk, evidence...)
	}
}

func guide(title, summary string, coverage string, notes string, sources []Source, fields []Field) Guide {
	return Guide{
		Title: title, Summary: summary,
		Coverage: Coverage{Level: coverage, Notes: notes},
		Sources:  sources, Fields: fields,
	}
}

func serializationField(path, title, description, gameUsage, editRole, editGuidance, risk string, evidence ...Source) Field {
	return Field{
		JSONPath:     path,
		Title:        title,
		Description:  description,
		GameUsage:    gameUsage,
		EditRole:     editRole,
		EditGuidance: editGuidance,
		Risk:         risk,
		Evidence:     evidence,
		Confidence:   ConfidenceSerializationOnly,
	}
}

func serializationFieldFrom(evidence ...Source) func(string, string, string, string, string, string, string) Field {
	return func(path, title, description, gameUsage, editRole, editGuidance, risk string) Field {
		return serializationField(path, title, description, gameUsage, editRole, editGuidance, risk, evidence...)
	}
}

func pattern(path, title, description, gameUsage, editRole, editGuidance string, evidence ...Source) FieldPattern {
	return FieldPattern{
		JSONPathPattern: path,
		Title:           title,
		Description:     description,
		GameUsage:       gameUsage,
		EditRole:        editRole,
		EditGuidance:    editGuidance,
		Evidence:        evidence,
		Confidence:      confidenceForEvidence(evidence),
	}
}

func preserveUnknownRule() Rule {
	return Rule{
		ID:       "respect-typed-and-binary-data",
		Severity: "error",
		Summary:  "Retain modeled values and semantic binary assets without inventing unsupported wire state.",
		Details:  "Keep typed fields whose purpose is unknown, hashes, IDs, versions, and byte arrays whose native meaning is asset data or an independently unrecognized virtual file. Unknown indexed slots, synthetic nil flags, ordering metadata, trailing bytes, and raw or base64 parse-failure fallbacks are not editing fields and must be rejected by the decoder.",
	}
}

func standardWorkflow(formatID string) []string {
	return []string{
		"Read meido://schemas/" + formatID + " and this guide before editing.",
		"Convert a real source file to editing JSON; do not invent identifiers, resource names, bone paths, hashes, or enum values.",
		"Change only the fields required by the objective, retain other typed values and semantic binary assets, and never invent fields absent from the schema.",
		"Call meido.validate_editing_json and convert back to native only after validation succeeds.",
		"Test the result in the target game build when the edit affects rendering, animation, physics, attachment, catalog lookup, or character state.",
	}
}

func standardWarnings() []string {
	return []string{
		"Schema validation proves structure and supported wire invariants, not that a referenced asset, bone, material, hash, ID, or enum exists in the target installation.",
		"A runtime_verified guide covers the cited paths and fields only; it does not make merged schema_only fields safe to reinterpret.",
		"Unprefixed review states record AI source review. The human_ prefix is reserved for explicit human approval and must never be inferred or added automatically.",
	}
}
