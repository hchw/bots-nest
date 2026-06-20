// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"testing"
)

func TestNewSkillEngine(t *testing.T) {
	engine := NewSkillEngine()
	if engine == nil {
		t.Fatal("engine 不应为 nil")
	}
}

func TestSkillRegister(t *testing.T) {
	engine := NewSkillEngine()
	skills := []Skill{
		{Name: "search", Description: "搜索", SystemPrompt: "你是一个搜索助手", Enabled: true},
	}
	engine.Register("bot1", skills)
	got := engine.GetSkills("bot1")
	if len(got) != 1 {
		t.Errorf("期望 1 个技能，得到 %d", len(got))
	}
}

func TestSkillLookup(t *testing.T) {
	engine := NewSkillEngine()
	engine.Register("bot1", []Skill{
		{Name: "搜索", Description: "搜索技能", SystemPrompt: "搜索助手", Enabled: true},
		{Name: "介绍", Description: "介绍技能", SystemPrompt: "介绍助手", Enabled: false},
	})
	skill := engine.Lookup("bot1", "搜索")
	if skill == nil {
		t.Fatal("应找到搜索技能")
	}
	if skill.Name != "搜索" {
		t.Errorf("期望 搜索，得到 %s", skill.Name)
	}
	skill2 := engine.Lookup("bot1", "介绍")
	if skill2 != nil {
		t.Fatal("禁用技能不应被 Lookup 找到")
	}
	skill3 := engine.Lookup("bot1", "不存在")
	if skill3 != nil {
		t.Fatal("不应找到不存在的技能")
	}
}
