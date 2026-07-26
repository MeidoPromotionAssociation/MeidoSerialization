package knowledgev1

import (
	"sort"
	"strings"
)

// reviewedProfiles 保存每个有效指南中经过源码审核的 profile 部分 / reviewedProfiles stores the source-reviewed profile portion of every effective guide
var reviewedProfiles = buildReviewedProfiles()

// buildReviewedProfiles 汇总 COM3D2、KCES 容器和载荷格式的源码审核 profile
// buildReviewedProfiles assembles source-reviewed profiles for COM3D2, KCES containers, and payload formats
func buildReviewedProfiles() map[string]Guide {
	profiles := make(map[string]Guide)
	registerProfiles(profiles, com3d2Profiles())
	registerProfiles(profiles, kcesProfiles())
	registerProfiles(profiles, kcesContainerProfiles())
	registerProfiles(profiles, kcesPayloadProfiles())
	return profiles
}

// registerProfiles 检测重复项并在加入总目录前完成每个 profile
// registerProfiles detects duplicates and finalizes each profile before adding it to the combined catalog
func registerProfiles(destination, source map[string]Guide) {
	for formatID, guide := range source {
		if _, exists := destination[formatID]; exists {
			panic("duplicate format-guide profile " + formatID)
		}
		destination[formatID] = finalizeProfile(formatID, guide)
	}
}

// finalizeProfile 填充稳定元数据并校验字段、证据、规则、值集合和命令引用
// finalizeProfile fills stable metadata and validates fields, evidence, rules, value sets, and command references
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

// finalizeValueSets 校验值集合完整性、唯一标识符、证据和置信等级
// finalizeValueSets validates value-set completeness, unique identifiers, evidence, and confidence levels
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

// validateCommandValueSetRefs 校验命令语义完整性及其值集合引用
// validateCommandValueSetRefs validates command-semantic completeness and value-set references
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

// confidenceForEvidence 根据游戏源码或实现源码证据推导默认置信等级
// confidenceForEvidence derives the default confidence level from game-source or implementation-source evidence
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

// requireGameEvidence 确保标记为已验证的结论至少引用一处游戏源码
// requireGameEvidence ensures that a conclusion marked verified cites at least one game-source location
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

// validateSources 校验证据种类、精确行范围及其所属源码树
// validateSources validates evidence kinds, exact line ranges, and source-tree ownership
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

// isKnownCoverageLevel 判断字符串是否为目录支持的覆盖等级
// isKnownCoverageLevel reports whether a string is a coverage level supported by the catalog
func isKnownCoverageLevel(level string) bool {
	switch level {
	case CoverageRuntimeVerified, CoverageSerializationVerified, CoverageSchemaOnly,
		CoverageHumanRuntimeVerified, CoverageHumanSerializationVerified:
		return true
	default:
		return false
	}
}

// isKnownConfidence 判断字符串是否为目录支持的字段置信等级
// isKnownConfidence reports whether a string is a field confidence level supported by the catalog
func isKnownConfidence(confidence string) bool {
	switch confidence {
	case ConfidenceVerified, ConfidenceSerializationOnly, ConfidenceSchemaOnly,
		ConfidenceHumanVerified, ConfidenceHumanSerializationOnly:
		return true
	default:
		return false
	}
}

// profileGuide 按稳定格式标识符查找源码审核指南 profile
// profileGuide looks up a source-reviewed guide profile by stable format identifier
func profileGuide(formatID string) (Guide, bool) {
	guide, found := reviewedProfiles[formatID]
	return guide, found
}

// profileFormats 返回按字典序排列的全部源码审核 profile 格式标识符
// profileFormats returns all source-reviewed profile format identifiers in lexical order
func profileFormats() []string {
	formats := make([]string, 0, len(reviewedProfiles))
	for formatID := range reviewedProfiles {
		formats = append(formats, formatID)
	}
	sort.Strings(formats)
	return formats
}

// source 构建引用游戏源码精确位置的证据
// source builds evidence referencing an exact game-source location
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

// implementationSource 构建引用本库实现源码精确位置的证据
// implementationSource builds evidence referencing an exact library-implementation source location
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

// field 构建根据所给证据自动推导置信等级的字段语义
// field builds field semantics with a confidence level derived from the supplied evidence
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

// fieldFrom 将共享证据绑定到可重复使用的字段构造函数
// fieldFrom binds shared evidence to a reusable field constructor
func fieldFrom(evidence ...Source) func(string, string, string, string, string, string, string) Field {
	return func(path, title, description, gameUsage, editRole, editGuidance, risk string) Field {
		return field(path, title, description, gameUsage, editRole, editGuidance, risk, evidence...)
	}
}

// guide 使用标题、摘要、覆盖信息、证据和字段构建基础 profile
// guide builds a base profile from a title, summary, coverage information, evidence, and fields
func guide(title, summary string, coverage string, notes string, sources []Source, fields []Field) Guide {
	return Guide{
		Title: title, Summary: summary,
		Coverage: Coverage{Level: coverage, Notes: notes},
		Sources:  sources, Fields: fields,
	}
}

// serializationField 构建明确只由序列化实现证据支持的字段语义
// serializationField builds field semantics explicitly supported only by serialization-implementation evidence
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

// serializationFieldFrom 将共享实现证据绑定到可重复使用的序列化字段构造函数
// serializationFieldFrom binds shared implementation evidence to a reusable serialization-field constructor
func serializationFieldFrom(evidence ...Source) func(string, string, string, string, string, string, string) Field {
	return func(path, title, description, gameUsage, editRole, editGuidance, risk string) Field {
		return serializationField(path, title, description, gameUsage, editRole, editGuidance, risk, evidence...)
	}
}

// pattern 构建根据所给证据自动推导置信等级的动态字段模式
// pattern builds a dynamic field pattern with a confidence level derived from the supplied evidence
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

// preserveUnknownRule 返回禁止丢失建模值或虚构未支持 wire 状态的标准规则
// preserveUnknownRule returns the standard rule forbidding loss of modeled values or invention of unsupported wire state
func preserveUnknownRule() Rule {
	return Rule{
		ID:       "respect-typed-and-binary-data",
		Severity: "error",
		Summary:  "Retain modeled values and semantic binary assets without inventing unsupported wire state.",
		Details:  "Keep typed fields whose purpose is unknown, hashes, IDs, versions, and byte arrays whose native meaning is asset data or an independently unrecognized virtual file. Unknown indexed slots, synthetic nil flags, ordering metadata, trailing bytes, and raw or base64 parse-failure fallbacks are not editing fields and must be rejected by the decoder.",
	}
}

// standardWorkflow 返回指定格式通用的无损编辑和验证步骤
// standardWorkflow returns common lossless editing and validation steps for a format
func standardWorkflow(formatID string) []string {
	return []string{
		"Read meido://schemas/" + formatID + " and this guide before editing.",
		"Convert a real source file to editing JSON; do not invent identifiers, resource names, bone paths, hashes, or enum values.",
		"Change only the fields required by the objective, retain other typed values and semantic binary assets, and never invent fields absent from the schema.",
		"Call meido.validate_editing_json and convert back to native only after validation succeeds.",
		"Test the result in the target game build when the edit affects rendering, animation, physics, attachment, catalog lookup, or character state.",
	}
}

// standardWarnings 返回适用于全部格式指南的能力边界警告
// standardWarnings returns capability-boundary warnings applicable to all format guides
func standardWarnings() []string {
	return []string{
		"Schema validation proves structure and supported wire invariants, not that a referenced asset, bone, material, hash, ID, or enum exists in the target installation.",
		"A runtime_verified guide covers the cited paths and fields only; it does not make merged schema_only fields safe to reinterpret.",
		"Unprefixed review states record AI source review. The human_ prefix is reserved for explicit human approval and must never be inferred or added automatically.",
	}
}
