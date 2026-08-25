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
	if guide.FormatVerification.Level == "" {
		guide.FormatVerification.Level = FormatVerificationSerializationVerified
	}
	if guide.FormatVerification.Authority == "" {
		if guide.FormatVerification.Level == FormatVerificationSchemaOnly {
			guide.FormatVerification.Authority = ReviewAuthorityGenerated
		} else {
			guide.FormatVerification.Authority = ReviewAuthorityAI
		}
	}
	validateFormatVerification(formatID, guide.FormatVerification)
	if guide.FormatVerification.Notes == "" {
		guide.FormatVerification.Notes = "The whole-file serialization contract was reviewed by AI. Field verification claims are independent: source_semantics records game-source review, game_behavior requires an actual runtime observation, and an empty verification object means schema-derived only."
	}
	validateSources(formatID, "guide", guide.Sources)
	for index := range guide.Fields {
		field := &guide.Fields[index]
		if field.SchemaPointer == "" && field.JSONPath == documentRootJSONPath {
			field.SchemaPointer = "#"
		}
		if field.SchemaPointer == "" && strings.Count(field.JSONPath, "/") == 1 {
			field.SchemaPointer = "#/properties/" + escapeJSONPointer(strings.TrimPrefix(field.JSONPath, "/"))
		}
		validateSources(formatID, "field "+field.JSONPath, field.Evidence)
		field.Verification = completeFieldVerification(field.Verification, field.Evidence, guide.FormatVerification)
		validateFieldVerification(formatID, "field "+field.JSONPath, field.Verification, field.Evidence, guide.FormatVerification)
		if field.Risk == "" {
			field.Risk = "medium"
		}
	}
	for index := range guide.FieldPatterns {
		fieldPattern := &guide.FieldPatterns[index]
		validateSources(formatID, "field pattern "+fieldPattern.JSONPathPattern, fieldPattern.Evidence)
		fieldPattern.Verification = completeFieldVerification(fieldPattern.Verification, fieldPattern.Evidence, guide.FormatVerification)
		validateFieldVerification(formatID, "field pattern "+fieldPattern.JSONPathPattern, fieldPattern.Verification, fieldPattern.Evidence, guide.FormatVerification)
	}
	guide.FieldCoverage = summarizeFieldCoverage(guide.Fields)
	finalizeValueSets(formatID, guide.ValueSets, guide.FormatVerification)
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

// finalizeValueSets 校验值集合完整性、唯一标识符、证据和独立认证 claim
// finalizeValueSets validates value-set completeness, unique identifiers, evidence, and independent verification claims
func finalizeValueSets(formatID string, valueSets []ValueSet, formatVerification FormatVerification) {
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
		validateSources(formatID, "value set "+valueSet.ID, valueSet.Evidence)
		valueSet.Verification = completeFieldVerification(valueSet.Verification, valueSet.Evidence, formatVerification)
		validateFieldVerification(formatID, "value set "+valueSet.ID, valueSet.Verification, valueSet.Evidence, formatVerification)
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

// verificationForEvidence 根据实现源码、游戏源码或实际运行证据推导独立认证
// verificationForEvidence derives independent claims from implementation-source, game-source, or actual-runtime evidence
func verificationForEvidence(evidence []Source) FieldVerification {
	verification := FieldVerification{}
	for _, item := range evidence {
		switch item.Kind {
		case SourceKindGame:
			verification.Serialization = verifiedClaim(ReviewAuthorityAI)
			verification.SourceSemantics = verifiedClaim(ReviewAuthorityAI)
		case SourceKindImplementation:
			verification.Serialization = verifiedClaim(ReviewAuthorityAI)
		case SourceKindRuntimeObservation:
			verification.GameBehavior = verifiedClaim(ReviewAuthorityAI)
		}
	}
	return verification
}

// completeFieldVerification 合并显式认证、证据推导认证和文件级序列化认证
// completeFieldVerification merges explicit claims, evidence-derived claims, and whole-file serialization verification
func completeFieldVerification(verification FieldVerification, evidence []Source, formatVerification FormatVerification) FieldVerification {
	inferred := verificationForEvidence(evidence)
	if verification.Serialization == nil {
		verification.Serialization = inferred.Serialization
	}
	if verification.SourceSemantics == nil {
		verification.SourceSemantics = inferred.SourceSemantics
	}
	if verification.GameBehavior == nil {
		verification.GameBehavior = inferred.GameBehavior
	}
	if verification.Serialization == nil && verification.SourceSemantics != nil {
		verification.Serialization = verifiedClaim(verification.SourceSemantics.Authority)
	}
	if verification.Serialization == nil && formatVerification.Level == FormatVerificationSerializationVerified {
		verification.Serialization = verifiedClaim(formatVerification.Authority)
	}
	return verification
}

// verifiedClaim 构建带有明确审阅主体的已认证 claim
// verifiedClaim builds a verified claim carrying an explicit reviewing authority
func verifiedClaim(authority string) *VerificationClaim {
	return &VerificationClaim{Status: VerificationStatusVerified, Authority: authority}
}

// validateFieldVerification 校验独立认证状态、包含关系及所需证据
// validateFieldVerification validates independent claims, inclusion rules, and required evidence
func validateFieldVerification(formatID, owner string, verification FieldVerification, evidence []Source, formatVerification FormatVerification) {
	for name, claim := range map[string]*VerificationClaim{
		"serialization":    verification.Serialization,
		"source_semantics": verification.SourceSemantics,
		"game_behavior":    verification.GameBehavior,
	} {
		if claim == nil {
			continue
		}
		if claim.Status != VerificationStatusVerified || !isReviewAuthority(claim.Authority) {
			panic("invalid " + name + " verification for " + owner + " in " + formatID)
		}
	}
	if verification.SourceSemantics != nil && verification.Serialization == nil {
		panic("source verification without serialization verification for " + owner + " in " + formatID)
	}
	if verification.Serialization != nil && verification.SourceSemantics == nil && formatVerification.Level != FormatVerificationSerializationVerified && !hasEvidenceKind(evidence, SourceKindImplementation) {
		panic("serialization verification without serialization evidence for " + owner + " in " + formatID)
	}
	if verification.SourceSemantics != nil && !hasEvidenceKind(evidence, SourceKindGame) {
		panic("source verification without game source for " + owner + " in " + formatID)
	}
	if verification.GameBehavior != nil && !hasEvidenceKind(evidence, SourceKindRuntimeObservation) {
		panic("game-behavior verification without runtime observation for " + owner + " in " + formatID)
	}
}

// hasEvidenceKind 判断证据列表是否包含指定种类
// hasEvidenceKind reports whether an evidence list contains the requested kind
func hasEvidenceKind(evidence []Source, kind string) bool {
	for _, item := range evidence {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

// validateSources 校验证据种类、精确行范围及路径约束
// validateSources validates evidence kinds, exact line ranges, and path constraints
func validateSources(formatID, owner string, sources []Source) {
	for _, item := range sources {
		if item.Kind != SourceKindGame && item.Kind != SourceKindImplementation && item.Kind != SourceKindRuntimeObservation {
			panic("invalid source kind " + item.Kind + " for " + owner + " in " + formatID)
		}
		path := strings.ReplaceAll(item.Path, "\\", "/")
		if item.GameVersion == "" || item.Observation == "" {
			panic("incomplete evidence for " + owner + " in " + formatID)
		}
		if item.Kind != SourceKindRuntimeObservation && (item.Path == "" || item.LineStart < 1 || item.LineEnd < item.LineStart) {
			panic("imprecise source for " + owner + " in " + formatID)
		}
		if strings.HasPrefix(path, "game/") {
			panic("evidence path must not include the game/ prefix for " + owner + " in " + formatID)
		}
	}
}

// validateFormatVerification 校验文件级认证等级与主体组合
// validateFormatVerification validates the whole-file verification level and authority combination
func validateFormatVerification(formatID string, verification FormatVerification) {
	switch verification.Level {
	case FormatVerificationSerializationVerified:
		if !isReviewAuthority(verification.Authority) {
			panic("invalid serialization verification authority " + verification.Authority + " for " + formatID)
		}
	case FormatVerificationSchemaOnly:
		if verification.Authority != ReviewAuthorityGenerated {
			panic("schema-only format verification must be generated for " + formatID)
		}
	default:
		panic("invalid format verification " + verification.Level + " for " + formatID)
	}
}

// isReviewAuthority 判断字符串是否为允许作出认证结论的主体
// isReviewAuthority reports whether a string identifies an authority allowed to make verification claims
func isReviewAuthority(authority string) bool {
	return authority == ReviewAuthorityAI || authority == ReviewAuthorityHuman
}

// summarizeFieldCoverage 统计精确字段的独立认证覆盖
// summarizeFieldCoverage counts independent verification coverage for exact fields
func summarizeFieldCoverage(fields []Field) FieldCoverage {
	summary := FieldCoverage{Total: uint32(len(fields))}
	for _, field := range fields {
		verification := field.Verification
		if verification.Serialization != nil {
			summary.SerializationVerified++
		}
		if verification.SourceSemantics != nil {
			summary.SourceVerified++
		}
		if verification.GameBehavior != nil {
			summary.GameBehaviorVerified++
		}
		if verification.Serialization == nil && verification.SourceSemantics == nil && verification.GameBehavior == nil {
			summary.SchemaDerived++
		}
	}
	return summary
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
	path = strings.ReplaceAll(path, "\\", "/")
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

// field 构建根据所给证据自动推导独立认证 claim 的字段语义
// field builds field semantics with independent verification claims derived from the supplied evidence
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
		Verification: verificationForEvidence(evidence),
	}
}

// fieldFrom 将共享证据绑定到可重复使用的字段构造函数
// fieldFrom binds shared evidence to a reusable field constructor
func fieldFrom(evidence ...Source) func(string, string, string, string, string, string, string) Field {
	return func(path, title, description, gameUsage, editRole, editGuidance, risk string) Field {
		return field(path, title, description, gameUsage, editRole, editGuidance, risk, evidence...)
	}
}

// guide 使用标题、摘要、文件认证、证据和字段构建基础 profile
// guide builds a base profile from a title, summary, whole-file verification, evidence, and fields
func guide(title, summary string, verification string, notes string, sources []Source, fields []Field) Guide {
	return Guide{
		Title: title, Summary: summary,
		FormatVerification: FormatVerification{Level: verification, Authority: ReviewAuthorityAI, Notes: notes},
		Sources:            sources, Fields: fields,
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
		Verification: FieldVerification{Serialization: verifiedClaim(ReviewAuthorityAI)},
	}
}

// pattern 构建根据所给证据自动推导独立认证 claim 的动态字段模式
// pattern builds a dynamic field pattern with independent verification claims derived from the supplied evidence
func pattern(path, title, description, gameUsage, editRole, editGuidance string, evidence ...Source) FieldPattern {
	return FieldPattern{
		JSONPathPattern: path,
		Title:           title,
		Description:     description,
		GameUsage:       gameUsage,
		EditRole:        editRole,
		EditGuidance:    editGuidance,
		Evidence:        evidence,
		Verification:    verificationForEvidence(evidence),
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
		"Whole-file serialization verification does not establish game semantics; inspect each field's independent verification claims before interpreting it.",
		"Every verification claim records an explicit AI or human authority. An empty verification object means the field is schema-derived and must not be interpreted from its name.",
	}
}
