// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"testing"

	"github.com/hchw/bots-nest/internal/config"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager("test-bot")
	if sm == nil {
		t.Fatal("sm 不应为 nil")
	}
	if sm.botID != "test-bot" {
		t.Errorf("期望 botID=test-bot，得到 %s", sm.botID)
	}
}

func newTestConfig() *config.Config {
	return &config.Config{SkillsDir: "skills"}
}

func TestNewBotManager(t *testing.T) {
	bm := NewBotManager(newTestConfig(), nil, nil)
	if bm == nil {
		t.Fatal("bm 不应为 nil")
	}
}

func TestBotManagerAddGet(t *testing.T) {
	bm := NewBotManager(newTestConfig(), nil, nil)
	bot := &BotInstance{ID: "bot1"}
	bm.AddBot("bot1", bot)
	got := bm.GetBot("bot1")
	if got == nil {
		t.Fatal("应找到 bot1")
	}
	if got.ID != "bot1" {
		t.Errorf("期望 ID=bot1，得到 %s", got.ID)
	}
}

func TestBotManagerRemove(t *testing.T) {
	bm := NewBotManager(newTestConfig(), nil, nil)
	bm.AddBot("bot1", &BotInstance{ID: "bot1"})
	bm.RemoveBot("bot1")
	got := bm.GetBot("bot1")
	if got != nil {
		t.Fatal("移除后应返回 nil")
	}
}

func TestBotManagerGetAll(t *testing.T) {
	bm := NewBotManager(newTestConfig(), nil, nil)
	bm.AddBot("bot1", &BotInstance{ID: "bot1"})
	bm.AddBot("bot2", &BotInstance{ID: "bot2"})
	all := bm.GetAll()
	if len(all) != 2 {
		t.Errorf("期望 2 个机器人，得到 %d", len(all))
	}
}
