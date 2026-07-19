// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package db

import (
	"log"
)

func Migrate() {
	if err := DB.AutoMigrate(
		&LLMProvider{},
		&MCP{},
		&Bot{},
		&Skill{},
		&GoJudgeTool{},
		&Session{},
		&Message{},
		&KnowledgeBase{},
		&ImportTask{},
		&BotKnowledgeBinding{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	dropOldColumns()
	log.Println("数据库迁移完成")
}

func dropOldColumns() {
	for _, col := range []string{"wecom_bot_id", "wecom_secret"} {
		if DB.Migrator().HasColumn(&Bot{}, col) {
			log.Printf("检测到旧字段 %s，正在移除...", col)
			if err := DB.Migrator().DropColumn(&Bot{}, col); err != nil {
				log.Fatalf("移除 %s 失败: %v", col, err)
			}
			log.Printf("旧字段 %s 已移除", col)
		}
	}
}
