// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package db

import (
	"log"
	"github.com/hchw/bots-nest/internal/agent"
	"github.com/hchw/bots-nest/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(cfg *config.Config) {
	var err error
	DB, err = gorm.Open(sqlite.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("获取底层 DB 失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)

	log.Println("数据库连接成功")
}

func SeedFromYAML(cfg *config.Config) {
	var count int64
	DB.Model(&LLMProvider{}).Count(&count)
	if count > 0 {
		log.Println("数据库已有数据，跳过种子数据导入")
		return
	}

	for _, p := range cfg.LLMProviders {
		DB.Create(&LLMProvider{
			ID:       p.Name,
			Name:     p.Name,
			Endpoint: p.Endpoint,
			APIKey:   p.APIKey,
			Enabled:  true,
		})
	}
	log.Printf("已导入 %d 个 LLM Provider", len(cfg.LLMProviders))

	for _, m := range cfg.MCPs {
		tools, err := agent.NewMCPClient(m.Name, m.Endpoint).DiscoverTools()
		if err != nil {
			log.Printf("[MCP] %s (%s) 自动发现工具失败: %v", m.Name, m.Endpoint, err)
		}
		DB.Create(&MCP{
			ID:       m.Name,
			Name:     m.Name,
			Endpoint: m.Endpoint,
			Tools:    tools,
			Enabled:  true,
		})
	}
	log.Printf("已导入 %d 个 MCP", len(cfg.MCPs))

	for _, b := range cfg.Bots {
		bot := Bot{
			ID:               b.Name,
			Name:             b.Name,
			WecomBotID:       b.WecomBotID,
			WecomSecret:      b.WecomSecret,
			LLMProviderID:    b.LLMProviderID,
			LLMModel:         b.LLMModel,
			MaxSessionTokens: b.MaxSessionTokens,
			Enabled:          b.Enabled,
		}
		if b.LLMTemperature != 0 {
			v := b.LLMTemperature
			bot.LLMTemperature = &v
		}
		if b.LLMMaxTokens != 0 {
			v := b.LLMMaxTokens
			bot.LLMMaxTokens = &v
		}
		DB.Create(&bot)

		for _, s := range b.Skills {
			DB.Create(&Skill{
				BotID:        bot.ID,
				Name:         s.Name,
				Description:  s.Description,
				SystemPrompt: s.SystemPrompt,
				Tools:        s.Tools,
				Enabled:      s.Enabled,
			})
		}
	}
	log.Printf("已导入 %d 个 Bot", len(cfg.Bots))
}
