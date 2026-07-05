// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package main

import (
	"encoding/json"
	"log"

	"github.com/hchw/bots-nest/internal/agent"
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

	// 补发现已有 MCP 的工具列表（首次启动或旧数据库升级）
	var mcps []db.MCP
	db.DB.Where("tools IS NULL OR tools = '' OR tools = '[]'").Find(&mcps)
	for _, m := range mcps {
		if m.Type == "command" {
			log.Printf("[MCP] 补发现工具: %s (%s)", m.Name, m.Command)
			var args []string
			if m.Args != "" {
				json.Unmarshal([]byte(m.Args), &args)
			}
			tools, err := agent.NewLocalMCPClient(m.Name, m.Command, args).DiscoverTools()
			if err != nil {
				log.Printf("[MCP] %s 发现失败: %v", m.Name, err)
				continue
			}
			db.DB.Model(&m).Update("tools", tools)
		} else {
			log.Printf("[MCP] 补发现工具: %s (%s)", m.Name, m.Endpoint)
			tools, err := agent.NewMCPClient(m.Name, m.Endpoint).DiscoverTools()
			if err != nil {
				log.Printf("[MCP] %s 发现失败: %v", m.Name, err)
				continue
			}
			db.DB.Model(&m).Update("tools", tools)
		}
	}

	log.Println("数据库初始化完成")

	botManager := bot.NewBotManager(cfg)
	botManager.LoadFromDB()

	r := gin.Default()

	handler := api.NewHandler(botManager, cfg)
	handler.RegisterRoutes(r)

	web.ServeStatic(r)

	log.Println("服务器启动在 :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
