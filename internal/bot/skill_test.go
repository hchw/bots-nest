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

func TestSkillMatch(t *testing.T) {
	engine := NewSkillEngine()
	engine.Register("bot1", []Skill{
		{Name: "搜索", Description: "搜索技能", SystemPrompt: "搜索助手", Enabled: true},
	})
	skill := engine.Match("bot1", "帮我搜索一下")
	if skill == nil {
		t.Fatal("应匹配到搜索技能")
	}
	if skill.Name != "搜索" {
		t.Errorf("期望匹配 搜索，得到 %s", skill.Name)
	}
}

func TestSkillNoMatch(t *testing.T) {
	engine := NewSkillEngine()
	engine.Register("bot1", []Skill{
		{Name: "search", Description: "搜索", SystemPrompt: "搜索助手", Enabled: true},
	})
	skill := engine.Match("bot1", "你好")
	if skill != nil {
		t.Fatal("不应匹配任何技能")
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"", "", true},
	}
	for _, tt := range tests {
		got := containsIgnoreCase(tt.s, tt.substr)
		if got != tt.expected {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, 期望 %v", tt.s, tt.substr, got, tt.expected)
		}
	}
}
