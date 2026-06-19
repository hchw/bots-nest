// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package db

import "log"

func Migrate() {
	if err := DB.AutoMigrate(
		&LLMProvider{},
		&MCP{},
		&Bot{},
		&Skill{},
		&Session{},
		&Message{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("数据库迁移完成")
}
