package knowledgev1

import (
	_ "embed"
	"strings"
)

// SkillMediaType 是格式编辑技能文档的媒体类型 / SkillMediaType is the media type of format-editing skill documents
const SkillMediaType = "text/markdown"

//go:embed editing.skill.md
var editingSkillTemplate string

// EditingSkill 使用格式、文件认证等级和当前写入策略呈现可移植编辑技能
// EditingSkill renders the portable editing skill with a format, whole-file verification level, and current write policy
func EditingSkill(formatID, verification, writePolicy string) string {
	value := strings.ReplaceAll(editingSkillTemplate, "{{FORMAT_ID}}", formatID)
	value = strings.ReplaceAll(value, "{{FORMAT_VERIFICATION}}", verification)
	if writePolicy = strings.TrimSpace(writePolicy); writePolicy != "" {
		value = strings.TrimRight(value, "\r\n") + "\n\n## Current write policy\n\n" + writePolicy + "\n"
	}
	return value
}
