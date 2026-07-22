// Package knowledgev1 contains the source-reviewed game-semantics guides used
// by the MCP and gRPC transports.
package knowledgev1

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Version   = "1.0.0"
	MediaType = "application/vnd.meido.format-guide+json"
	IDPrefix  = "urn:meido-serialization:format-guide:v1:"

	CoverageRuntimeVerified       = "runtime_verified"
	CoverageSerializationVerified = "serialization_verified"
	CoverageSchemaOnly            = "schema_only"

	ConfidenceVerified          = "verified"
	ConfidenceSerializationOnly = "serialization_only"
	ConfidenceSchemaOnly        = "schema_only"

	// HumanReviewPrefix is reserved for states explicitly approved by a human.
	// Automated generation and AI source review must never add this prefix.
	HumanReviewPrefix = "human_"

	CoverageHumanRuntimeVerified       = HumanReviewPrefix + CoverageRuntimeVerified
	CoverageHumanSerializationVerified = HumanReviewPrefix + CoverageSerializationVerified
	ConfidenceHumanVerified            = HumanReviewPrefix + ConfidenceVerified
	ConfidenceHumanSerializationOnly   = HumanReviewPrefix + ConfidenceSerializationOnly

	SourceKindGame           = "game_source"
	SourceKindImplementation = "implementation_source"
)

type Document struct {
	FormatID  string
	Version   string
	ID        string
	MediaType string
	SHA256    string
	SchemaID  string
	Coverage  string
	JSON      []byte
}

type Guide struct {
	ID            string         `json:"$id"`
	FormatID      string         `json:"format_id"`
	Version       string         `json:"guide_version"`
	SchemaURI     string         `json:"schema_uri"`
	SchemaID      string         `json:"schema_id,omitempty"`
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	Coverage      Coverage       `json:"coverage"`
	Sources       []Source       `json:"sources,omitempty"`
	Fields        []Field        `json:"fields,omitempty"`
	FieldPatterns []FieldPattern `json:"field_patterns,omitempty"`
	Rules         []Rule         `json:"editing_rules,omitempty"`
	Commands      []Command      `json:"command_semantics,omitempty"`
	ValueSets     []ValueSet     `json:"value_sets,omitempty"`
	Invariants    []string       `json:"invariants,omitempty"`
	Workflow      []string       `json:"editing_workflow,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}

type Coverage struct {
	Level          string `json:"level"`
	ReviewedFields int    `json:"reviewed_fields,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

type Source struct {
	Kind        string `json:"kind"`
	GameVersion string `json:"game_version"`
	Path        string `json:"path"`
	Symbol      string `json:"symbol,omitempty"`
	LineStart   int    `json:"line_start,omitempty"`
	LineEnd     int    `json:"line_end,omitempty"`
	Observation string `json:"observation"`
}

type Field struct {
	JSONPath      string      `json:"json_path"`
	SchemaPointer string      `json:"schema_pointer,omitempty"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	GameUsage     string      `json:"game_usage"`
	EditRole      string      `json:"edit_role"`
	EditGuidance  string      `json:"edit_guidance"`
	Risk          string      `json:"risk,omitempty"`
	Constraints   []string    `json:"constraints,omitempty"`
	Recommended   *Range      `json:"recommended_range,omitempty"`
	EnumValues    []EnumValue `json:"enum_values,omitempty"`
	Evidence      []Source    `json:"evidence,omitempty"`
	Confidence    string      `json:"confidence"`
}

type FieldPattern struct {
	JSONPathPattern string   `json:"json_path_pattern"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	GameUsage       string   `json:"game_usage"`
	EditRole        string   `json:"edit_role"`
	EditGuidance    string   `json:"edit_guidance"`
	Constraints     []string `json:"constraints,omitempty"`
	Evidence        []Source `json:"evidence,omitempty"`
	Confidence      string   `json:"confidence"`
}

type Range struct {
	Minimum float64 `json:"minimum"`
	Maximum float64 `json:"maximum"`
}

type EnumValue struct {
	Value   int    `json:"value"`
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

type Rule struct {
	ID        string   `json:"id"`
	AppliesTo []string `json:"applies_to,omitempty"`
	Severity  string   `json:"severity"`
	Summary   string   `json:"summary"`
	Details   string   `json:"details"`
	Evidence  []Source `json:"evidence,omitempty"`
}

type Command struct {
	Name         string        `json:"name"`
	Aliases      []string      `json:"aliases,omitempty"`
	Contexts     []string      `json:"contexts"`
	Forms        []CommandForm `json:"forms"`
	GameEffect   string        `json:"game_effect"`
	EditGuidance string        `json:"edit_guidance"`
	Risk         string        `json:"risk"`
	Evidence     []Source      `json:"evidence,omitempty"`
}

type CommandForm struct {
	Syntax     string            `json:"syntax"`
	ReviewedIn []string          `json:"reviewed_in"`
	Arguments  []CommandArgument `json:"arguments,omitempty"`
	Notes      string            `json:"notes,omitempty"`
}

type CommandArgument struct {
	Position      int      `json:"position"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	ValueSetRefs  []string `json:"value_set_refs,omitempty"`
	Required      bool     `json:"required"`
	Repeatable    bool     `json:"repeatable,omitempty"`
	Default       string   `json:"default,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	Description   string   `json:"description"`
}

type ValueSet struct {
	ID           string          `json:"id"`
	CSharpType   string          `json:"csharp_type"`
	Description  string          `json:"description"`
	EditGuidance string          `json:"edit_guidance"`
	ReviewedIn   []string        `json:"reviewed_in"`
	Values       []ValueSetValue `json:"values"`
	Evidence     []Source        `json:"evidence,omitempty"`
	Confidence   string          `json:"confidence"`
}

type ValueSetValue struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
}

func Lookup(formatID string) (Document, bool, error) {
	id := strings.ToLower(strings.TrimSpace(formatID))
	if id == "" || strings.ContainsAny(id, `/\\`) {
		return Document{}, false, nil
	}
	if profile, found := profileGuide(id); found {
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return Document{}, false, fmt.Errorf("encode format-guide profile %s: %w", id, err)
		}
		data = append(data, '\n')
		digest := sha256.Sum256(data)
		return Document{
			FormatID:  id,
			Version:   profile.Version,
			ID:        profile.ID,
			MediaType: MediaType,
			SHA256:    fmt.Sprintf("%x", digest[:]),
			SchemaID:  profile.SchemaID,
			Coverage:  profile.Coverage.Level,
			JSON:      data,
		}, true, nil
	}
	return Document{}, false, nil
}

func Decode(formatID string) (Guide, bool, error) {
	document, found, err := Lookup(formatID)
	if err != nil || !found {
		return Guide{}, found, err
	}
	var guide Guide
	if err := json.Unmarshal(document.JSON, &guide); err != nil {
		return Guide{}, false, fmt.Errorf("decode guide %s: %w", formatID, err)
	}
	return guide, true, nil
}

func Formats() ([]string, error) {
	return profileFormats(), nil
}

// IsHumanReviewed reports whether a coverage or field-confidence state carries
// explicit human approval. An unprefixed verified state may come from AI source
// review and must not be presented as human-reviewed.
func IsHumanReviewed(state string) bool {
	return strings.HasPrefix(state, HumanReviewPrefix)
}
