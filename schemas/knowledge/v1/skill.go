package knowledgev1

import (
	_ "embed"
	"strings"
)

const SkillMediaType = "text/markdown"

//go:embed editing.skill.md
var editingSkillTemplate string

func EditingSkill(formatID, coverage, writePolicy string) string {
	value := strings.ReplaceAll(editingSkillTemplate, "{{FORMAT_ID}}", formatID)
	value = strings.ReplaceAll(value, "{{COVERAGE}}", coverage)
	if writePolicy = strings.TrimSpace(writePolicy); writePolicy != "" {
		value = strings.TrimRight(value, "\r\n") + "\n\n## Current write policy\n\n" + writePolicy + "\n"
	}
	return value
}
