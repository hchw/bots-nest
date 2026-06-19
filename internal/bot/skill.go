// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

type SkillEngine struct {
	skills map[string][]Skill
}

type Skill struct {
	Name         string
	Description  string
	SystemPrompt string
	Tools        string
	Enabled      bool
}

func NewSkillEngine() *SkillEngine {
	return &SkillEngine{
		skills: make(map[string][]Skill),
	}
}

func (e *SkillEngine) Register(botID string, skills []Skill) {
	e.skills[botID] = skills
}

func (e *SkillEngine) GetSkills(botID string) []Skill {
	return e.skills[botID]
}

func (e *SkillEngine) Match(botID, message string) *Skill {
	for _, skill := range e.skills[botID] {
		if !skill.Enabled {
			continue
		}
		if containsIgnoreCase(message, skill.Name) {
			return &skill
		}
	}
	return nil
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return stringsContains(s, substr)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b[i] = s[i] + 32
		} else {
			b[i] = s[i]
		}
	}
	return string(b)
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsFunc(s, substr)
}

func containsFunc(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
