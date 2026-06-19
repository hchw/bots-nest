// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package main

import (
	"log"
	"github.com/hchw/bots-nest/internal/api"
	"github.com/hchw/bots-nest/internal/bot"
	"github.com/hchw/bots-nest/internal/config"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/web"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("加载配置...")
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置加载成功")

	log.Println("初始化数据库...")
	db.Init(cfg)
	db.Migrate()
	db.SeedFromYAML(cfg)
	log.Println("数据库初始化完成")

	botManager := bot.NewBotManager()
	botManager.LoadFromDB()

	r := gin.Default()

	handler := api.NewHandler(botManager)
	handler.RegisterRoutes(r)

	web.ServeStatic(r)

	log.Println("服务器启动在 :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
