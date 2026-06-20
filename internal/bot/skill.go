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

func (e *SkillEngine) Lookup(botID, name string) *Skill {
	for _, skill := range e.skills[botID] {
		if skill.Enabled && skill.Name == name {
			return &skill
		}
	}
	return nil
}
