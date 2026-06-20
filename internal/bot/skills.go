package bot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type builtinSkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Tools       string `yaml:"tools"`
	Enabled     bool   `yaml:"enabled"`
}

func LoadBuiltinSkills(skillsDir string) ([]Skill, error) {
	pattern := filepath.Join(skillsDir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("扫描内置 Skill 目录失败: %w", err)
	}

	var skills []Skill
	for _, path := range matches {
		skill, err := parseBuiltinSkillFile(path)
		if err != nil {
			fmt.Printf("[内置 Skill] 跳过 %s: %v\n", path, err)
			continue
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func parseBuiltinSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)

	const delim = "---"
	firstDelim := strings.Index(content, delim)
	if firstDelim != 0 {
		return Skill{}, fmt.Errorf("缺少 YAML frontmatter 起始分隔符")
	}

	rest := content[len(delim):]
	secondDelim := strings.Index(rest, delim)
	if secondDelim < 0 {
		return Skill{}, fmt.Errorf("缺少 YAML frontmatter 结束分隔符")
	}

	yamlPart := rest[:secondDelim]
	body := strings.TrimSpace(rest[secondDelim+len(delim):])

	var meta builtinSkillMeta
	if err := yaml.Unmarshal([]byte(yamlPart), &meta); err != nil {
		return Skill{}, fmt.Errorf("解析 YAML frontmatter 失败: %w", err)
	}

	if meta.Name == "" {
		return Skill{}, fmt.Errorf("YAML frontmatter 缺少 name 字段")
	}

	return Skill{
		Name:         meta.Name,
		Description:  meta.Description,
		SystemPrompt: body,
		Tools:        meta.Tools,
		Enabled:      meta.Enabled,
	}, nil
}
