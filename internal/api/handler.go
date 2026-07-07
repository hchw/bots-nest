// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"github.com/hchw/bots-nest/internal/agent"
	"github.com/hchw/bots-nest/internal/bot"
	"github.com/hchw/bots-nest/internal/config"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/knowledge"
	"github.com/hchw/bots-nest/internal/llm"
	"github.com/hchw/bots-nest/internal/skilltool"
	"github.com/hchw/bots-nest/internal/ws"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type Handler struct {
	botManager     *bot.BotManager
	cfg            *config.Config
	weaviateClient *knowledge.WeaviateClient
	wsHub          *ws.Hub
	importManager  *knowledge.ImportTaskManager
}

func NewHandler(bm *bot.BotManager, cfg *config.Config, wc *knowledge.WeaviateClient, hub *ws.Hub, im *knowledge.ImportTaskManager) *Handler {
	return &Handler{
		botManager:     bm,
		cfg:            cfg,
		weaviateClient: wc,
		wsHub:          hub,
		importManager:  im,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/llm-providers", h.listLLMProviders)
		api.GET("/llm-providers/:id/models", h.getLLMProviderModels)
		api.POST("/llm-providers", h.createLLMProvider)
		api.PUT("/llm-providers/:id", h.updateLLMProvider)
		api.DELETE("/llm-providers/:id", h.deleteLLMProvider)
		api.POST("/llm-providers/:id/refresh", h.refreshLLMProviderModels)

		api.GET("/mcps", h.listMCPs)
		api.POST("/mcps", h.createMCP)
		api.PUT("/mcps/:id", h.updateMCP)
		api.DELETE("/mcps/:id", h.deleteMCP)

		api.GET("/bots", h.listBots)
		api.GET("/bots/:id", h.getBot)
		api.POST("/bots", h.createBot)
		api.PUT("/bots/:id", h.updateBot)
		api.DELETE("/bots/:id", h.deleteBot)
		api.POST("/bots/:id/start", h.startBot)
		api.POST("/bots/:id/stop", h.stopBot)

		api.GET("/bots/:id/sessions", h.listBotSessions)
		api.GET("/bots/:id/skills", h.listBotSkills)
		api.POST("/bots/:id/skills", h.createSkill)
		api.PUT("/bots/:id/skills/:skillId", h.updateSkill)
		api.DELETE("/bots/:id/skills/:skillId", h.deleteSkill)

		toolHandler := skilltool.NewToolHandler(h.cfg.GoJudgeEndpoint)
		toolGroup := api.Group("/bots/:id/skills/:skillId/tools")
		toolHandler.RegisterRoutes(toolGroup)

		api.POST("/bots/:id/polish-code", h.polishCode)

		api.GET("/knowledge-bases", h.listKnowledgeBases)
		api.POST("/knowledge-bases", h.createKnowledgeBase)
		api.GET("/knowledge-bases/:id", h.getKnowledgeBase)
		api.PUT("/knowledge-bases/:id", h.updateKnowledgeBase)
		api.DELETE("/knowledge-bases/:id", h.deleteKnowledgeBase)
		api.POST("/knowledge-bases/:id/upload", h.uploadFile)
		api.GET("/knowledge-bases/:id/tasks", h.listImportTasks)
		api.GET("/import-tasks/:id", h.getImportTask)
		api.GET("/bots/:id/bindings", h.getBotBindings)
		api.PUT("/bots/:id/bindings", h.updateBotBindings)

		api.GET("/sessions/:key", h.getSession)
		api.POST("/sessions/:key/expire", h.expireSession)
		api.DELETE("/sessions/:key", h.deleteSession)
	}
}

func (h *Handler) listLLMProviders(c *gin.Context) {
	var providers []db.LLMProvider
	db.DB.Find(&providers)
	c.JSON(http.StatusOK, providers)
}

type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (h *Handler) getLLMProviderModels(c *gin.Context) {
	id := c.Param("id")
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", id).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+provider.APIKey).
		Get(provider.Endpoint + "/models")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []string{}})
		return
	}

	var modelsResp openAIModelsResponse
	if err := json.Unmarshal(resp.Body(), &modelsResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []string{}})
		return
	}

	models := make([]string, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = m.ID
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func fetchProviderModels(provider *db.LLMProvider) []string {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+provider.APIKey).
		Get(provider.Endpoint + "/models")
	if err != nil {
		return nil
	}

	var modelsResp openAIModelsResponse
	if err := json.Unmarshal(resp.Body(), &modelsResp); err != nil {
		return nil
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}
	return models
}

func (h *Handler) refreshLLMProviderModels(c *gin.Context) {
	id := c.Param("id")
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", id).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	models := fetchProviderModels(&provider)
	if models == nil {
		c.JSON(http.StatusOK, gin.H{"models": []string{}, "warning": "调用 /models 失败，缓存未更新"})
		return
	}

	data, _ := json.Marshal(models)
	db.DB.Model(&provider).Update("models", string(data))
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handler) listMCPs(c *gin.Context) {
	var mcps []db.MCP
	db.DB.Find(&mcps)
	c.JSON(http.StatusOK, mcps)
}

func (h *Handler) listBots(c *gin.Context) {
	var bots []db.Bot
	db.DB.Find(&bots)
	c.JSON(http.StatusOK, bots)
}

func (h *Handler) getBot(c *gin.Context) {
	id := c.Param("id")
	var botRecord db.Bot
	if err := db.DB.Where("id = ?", id).First(&botRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}
	c.JSON(http.StatusOK, botRecord)
}

func (h *Handler) listBotSessions(c *gin.Context) {
	botID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	sm := bot.NewSessionManager(botID)
	sessions, total, err := sm.GetSessions(botID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"page_size": pageSize,
	})
}

func (h *Handler) listBotSkills(c *gin.Context) {
		botID := c.Param("id")

		builtinSkills, err := bot.LoadBuiltinSkills(h.cfg.SkillsDir)
		if err != nil {
			log.Printf("[API] 加载内置 Skill 失败: %v", err)
		}
		for _, s := range builtinSkills {
			var existing db.Skill
			result := db.DB.Where("bot_id = ? AND name = ?", botID, s.Name).First(&existing)
			if result.Error != nil {
				db.DB.Create(&db.Skill{
					BotID:        botID,
					Name:         s.Name,
					Description:  s.Description,
					SystemPrompt: s.SystemPrompt,
					Tools:        s.Tools,
					Enabled:      s.Enabled,
				})
			} else {
				db.DB.Model(&existing).Updates(map[string]interface{}{
					"description":   s.Description,
					"system_prompt": s.SystemPrompt,
					"tools":         s.Tools,
				})
			}
		}

		var dbSkills []db.Skill
		db.DB.Where("bot_id = ?", botID).Find(&dbSkills)
		c.JSON(http.StatusOK, dbSkills)
	}

func (h *Handler) polishCode(c *gin.Context) {
	botID := c.Param("id")

	var req struct {
		Prompt   string `json:"prompt" binding:"required"`
		Language string `json:"language" binding:"required"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var botRecord db.Bot
	if err := db.DB.Where("id = ?", botID).First(&botRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "机器人未找到"})
		return
	}

	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", botRecord.LLMProviderID).First(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM Provider 未找到"})
		return
	}

	langDisplay := req.Language
	if langDisplay == "" {
		langDisplay = "python3"
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的代码生成助手。根据用户的想法生成 %s 语言的可执行代码。

要求：
1. 代码必须是完整、可执行的，不要包含任何额外的说明文字
2. 如果需要输入参数，使用 stdin 读取（input() 或类似方式）
3. 如果需要输出结果，使用 stdout 打印
4. 代码应当健壮，包含基本的错误处理
5. 不要使用外部库（仅使用标准库）
6. 代码长度控制在 500 行以内
7. 代码将通过 go-judge 执行系统运行，需要符合其沙箱执行环境要求`, langDisplay)

	userPrompt := fmt.Sprintf(`请根据以下需求对代码进行润色和改进。

当前代码：
%s

改进需求：
%s

请只返回改进后的完整代码，不要包含任何其他说明或 markdown 格式。`, req.Code, req.Prompt)

	if req.Code == "" {
		userPrompt = fmt.Sprintf(`请生成 %s 语言的代码，实现以下功能：

%s

请只返回代码，不要包含任何其他说明或 markdown 格式。`, langDisplay, req.Prompt)
	}

	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, botRecord.LLMModel)
	llmClient.Temperature = 0.3
	if botRecord.LLMMaxTokens != nil && *botRecord.LLMMaxTokens > 0 {
		llmClient.MaxTokens = *botRecord.LLMMaxTokens
	}

	resp, err := llmClient.Chat([]llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "LLM 调用失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": stripCodeFences(resp.Content)})
}

func stripCodeFences(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "```") {
		if idx := strings.Index(code[3:], "\n"); idx >= 0 {
			code = code[3+idx+1:]
		} else {
			code = code[3:]
		}
	}
	if strings.HasSuffix(code, "```") {
		code = code[:len(code)-3]
	}
	return strings.TrimSpace(code)
}

func (h *Handler) getSession(c *gin.Context) {
	key := c.Param("key")
	var session db.Session
	if err := db.DB.Where("session_key = ?", key).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话未找到"})
		return
	}

	var messages []db.Message
	db.DB.Where("session_key = ?", key).Order("created_at ASC").Find(&messages)

	c.JSON(http.StatusOK, gin.H{
		"session":  session,
		"messages": messages,
	})
}

func (h *Handler) expireSession(c *gin.Context) {
	key := c.Param("key")
	sm := bot.NewSessionManager("")
	if err := sm.ClearSession(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "会话已过期"})
}

func (h *Handler) deleteSession(c *gin.Context) {
	key := c.Param("key")
	sm := bot.NewSessionManager("")
	if err := sm.DeleteSession(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "会话已删除"})
}

func (h *Handler) createLLMProvider(c *gin.Context) {
	var req struct {
		ID       string `json:"id" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
		APIKey   string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	provider := db.LLMProvider{
		ID:       req.ID,
		Name:     req.Name,
		Endpoint: strings.TrimRight(req.Endpoint, "/"),
		APIKey:   req.APIKey,
		Enabled:  true,
	}
	if err := db.DB.Create(&provider).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	if models := fetchProviderModels(&provider); models != nil {
		data, _ := json.Marshal(models)
		db.DB.Model(&provider).Update("models", string(data))
		provider.Models = string(data)
	}
	c.JSON(http.StatusCreated, provider)
}

func (h *Handler) updateLLMProvider(c *gin.Context) {
	id := c.Param("id")
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", id).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Endpoint != "" {
		updates["endpoint"] = strings.TrimRight(req.Endpoint, "/")
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	needsRefresh := req.Endpoint != "" || req.APIKey != ""

	if err := db.DB.Model(&provider).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	if needsRefresh {
		db.DB.Where("id = ?", id).First(&provider)
		if models := fetchProviderModels(&provider); models != nil {
			data, _ := json.Marshal(models)
			db.DB.Model(&provider).Update("models", string(data))
			provider.Models = string(data)
		}
	}

	db.DB.Where("id = ?", id).First(&provider)
	c.JSON(http.StatusOK, provider)
}

func (h *Handler) deleteLLMProvider(c *gin.Context) {
	id := c.Param("id")
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", id).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	var botCount int64
	db.DB.Model(&db.Bot{}).Where("llm_provider_id = ?", id).Count(&botCount)
	if botCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("有 %d 个机器人绑定此 Provider，无法删除", botCount)})
		return
	}

	db.DB.Delete(&provider)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func autoDiscoverTools(mcpType, endpoint string, command string, args []string, env ...map[string]string) (string, error) {
	if mcpType == "command" {
		client := agent.NewLocalMCPClient("discovery", command, args, env...)
		result, err := client.DiscoverTools()
		if err != nil {
			log.Printf("[MCP] DiscoverTools 失败 command=%s: %v", command, err)
			return "[]", err
		}
		log.Printf("[MCP] DiscoverTools 成功 command=%s: %s", command, result)
		return result, nil
	}
	client := agent.NewMCPClient("discovery", endpoint)
	result, err := client.DiscoverTools()
	if err != nil {
		log.Printf("[MCP] DiscoverTools 失败 %s: %v", endpoint, err)
		return "[]", err
	}
	log.Printf("[MCP] DiscoverTools 成功 %s: %s", endpoint, result)
	return result, nil
}

func (h *Handler) createMCP(c *gin.Context) {
	var req struct {
		ID       string            `json:"id" binding:"required"`
		Name     string            `json:"name" binding:"required"`
		Type     string            `json:"type"`
		Endpoint string            `json:"endpoint"`
		Command  string            `json:"command"`
		Args     []string          `json:"args"`
		Env      map[string]string `json:"env"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Type == "" {
		req.Type = "url"
	}

	var (
		mcp       db.MCP
		discErr   string
	)
	switch req.Type {
	case "command":
		if req.Command == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "command 类型必须提供 command 字段"})
			return
		}
		argsStr := ""
		if len(req.Args) > 0 {
			data, _ := json.Marshal(req.Args)
			argsStr = string(data)
		}
		envStr := ""
		if len(req.Env) > 0 {
			data, _ := json.Marshal(req.Env)
			envStr = string(data)
		}
		tools, err := autoDiscoverTools("command", "", req.Command, req.Args, req.Env)
		if err != nil {
			discErr = err.Error()
		}
		mcp = db.MCP{
			ID:      req.ID,
			Name:    req.Name,
			Type:    "command",
			Command: req.Command,
			Args:    argsStr,
			Env:     envStr,
			Tools:   tools,
			Enabled: true,
		}
	default:
		if req.Endpoint == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url 类型必须提供 endpoint 字段"})
			return
		}
		endpoint := strings.TrimRight(req.Endpoint, "/")
		tools, err := autoDiscoverTools("url", endpoint, "", nil)
		if err != nil {
			discErr = err.Error()
		}
		mcp = db.MCP{
			ID:       req.ID,
			Name:     req.Name,
			Type:     "url",
			Endpoint: endpoint,
			Tools:    tools,
			Enabled:  true,
		}
	}

	if err := db.DB.Create(&mcp).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	resp := gin.H{"mcp": mcp}
	if discErr != "" {
		resp["warning"] = "工具自动发现失败: " + discErr
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) updateMCP(c *gin.Context) {
	id := c.Param("id")
	var mcp db.MCP
	if err := db.DB.Where("id = ?", id).First(&mcp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	var req struct {
		Name     string            `json:"name"`
		Type     string            `json:"type"`
		Endpoint string            `json:"endpoint"`
		Command  string            `json:"command"`
		Args     []string          `json:"args"`
		Env      map[string]string `json:"env"`
		Enabled  *bool             `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var discErr string
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Endpoint != "" {
		endpoint := strings.TrimRight(req.Endpoint, "/")
		updates["endpoint"] = endpoint
		updates["command"] = ""
		updates["args"] = ""
		updates["env"] = ""
		tools, err := autoDiscoverTools("url", endpoint, "", nil)
		if err != nil {
			discErr = err.Error()
		}
		updates["tools"] = tools
	}
	if req.Command != "" {
		updates["command"] = req.Command
		updates["endpoint"] = ""
		argsStr := ""
		if len(req.Args) > 0 {
			data, _ := json.Marshal(req.Args)
			argsStr = string(data)
		}
		updates["args"] = argsStr
		tools, err := autoDiscoverTools("command", "", req.Command, req.Args, req.Env)
		if err != nil {
			discErr = err.Error()
		}
		updates["tools"] = tools
	}
	if req.Env != nil {
		envStr := ""
		if len(req.Env) > 0 {
			data, _ := json.Marshal(req.Env)
			envStr = string(data)
		}
		updates["env"] = envStr
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := db.DB.Model(&mcp).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	db.DB.Where("id = ?", id).First(&mcp)

	resp := gin.H{"mcp": mcp}
	if discErr != "" {
		resp["warning"] = "工具自动发现失败: " + discErr
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) deleteMCP(c *gin.Context) {
	id := c.Param("id")
	var mcp db.MCP
	if err := db.DB.Where("id = ?", id).First(&mcp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}
	db.DB.Delete(&mcp)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) createBot(c *gin.Context) {
	var req struct {
		ID               string   `json:"id" binding:"required"`
		Name             string   `json:"name" binding:"required"`
		WecomBotID       string   `json:"wecom_bot_id" binding:"required"`
		WecomSecret      string   `json:"wecom_secret" binding:"required"`
		LLMProviderID    string   `json:"llm_provider_id" binding:"required"`
		LLMModel         string   `json:"llm_model"`
		LLMTemperature   *float64 `json:"llm_temperature"`
		LLMMaxTokens     *int     `json:"llm_max_tokens"`
		MaxSessionTokens int      `json:"max_session_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	bot := db.Bot{
		ID:               req.ID,
		Name:             req.Name,
		Status:           "disconnected",
		WecomBotID:       req.WecomBotID,
		WecomSecret:      req.WecomSecret,
		LLMProviderID:    req.LLMProviderID,
		LLMModel:         req.LLMModel,
		LLMTemperature:   req.LLMTemperature,
		LLMMaxTokens:     req.LLMMaxTokens,
		MaxSessionTokens: req.MaxSessionTokens,
		Enabled:          true,
	}
	if bot.LLMModel == "" {
		bot.LLMModel = "gpt-4o"
	}
	if bot.MaxSessionTokens == 0 {
		bot.MaxSessionTokens = 4096
	}

	if err := db.DB.Create(&bot).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, bot)
}

func (h *Handler) updateBot(c *gin.Context) {
	id := c.Param("id")
	var botRecord db.Bot
	if err := db.DB.Where("id = ?", id).First(&botRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	var req struct {
		Name             *string  `json:"name"`
		WecomBotID       *string  `json:"wecom_bot_id"`
		WecomSecret      *string  `json:"wecom_secret"`
		LLMProviderID    *string  `json:"llm_provider_id"`
		LLMModel         *string  `json:"llm_model"`
		LLMTemperature   *float64 `json:"llm_temperature"`
		LLMMaxTokens     *int     `json:"llm_max_tokens"`
		MaxSessionTokens *int     `json:"max_session_tokens"`
		Enabled          *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.WecomBotID != nil {
		updates["wecom_bot_id"] = *req.WecomBotID
	}
	if req.WecomSecret != nil {
		updates["wecom_secret"] = *req.WecomSecret
	}
	if req.LLMProviderID != nil {
		updates["llm_provider_id"] = *req.LLMProviderID
	}
	if req.LLMModel != nil {
		updates["llm_model"] = *req.LLMModel
	}
	if req.LLMTemperature != nil {
		updates["llm_temperature"] = *req.LLMTemperature
	}
	if req.LLMMaxTokens != nil {
		updates["llm_max_tokens"] = *req.LLMMaxTokens
	}
	if req.MaxSessionTokens != nil {
		updates["max_session_tokens"] = *req.MaxSessionTokens
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := db.DB.Model(&botRecord).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	wasConnected := h.botManager.GetBot(id) != nil
	if wasConnected {
		log.Printf("[API] 更新机器人 %s: 旧实例正在运行，执行滚动重启", id)
		if err := h.botManager.StopBot(id); err != nil {
			log.Printf("[API] 更新机器人 %s StopBot 错误: %v", id, err)
		}
		if err := h.botManager.StartBot(id); err != nil {
			log.Printf("[API] 更新机器人 %s StartBot 错误: %v", id, err)
		}
	} else {
		log.Printf("[API] 更新机器人 %s: 旧实例未运行，跳过重启", id)
	}

	db.DB.Where("id = ?", id).First(&botRecord)
	c.JSON(http.StatusOK, botRecord)
}

func (h *Handler) deleteBot(c *gin.Context) {
	id := c.Param("id")
	var botRecord db.Bot
	if err := db.DB.Where("id = ?", id).First(&botRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	h.botManager.StopBot(id)

	tx := db.DB.Begin()
	tx.Where("bot_id = ?", id).Delete(&db.Skill{})
	tx.Where("bot_id = ?", id).Delete(&db.Session{})
	tx.Where("session_key IN (SELECT session_key FROM sessions WHERE bot_id = ?)", id).Delete(&db.Message{})
	tx.Delete(&botRecord)
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) startBot(c *gin.Context) {
	id := c.Param("id")
	if err := h.botManager.StartBot(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "机器人已启动"})
}

func (h *Handler) stopBot(c *gin.Context) {
	id := c.Param("id")
	if err := h.botManager.StopBot(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "机器人已停止"})
}

func (h *Handler) createSkill(c *gin.Context) {
	botID := c.Param("id")
	var botRecord db.Bot
	if err := db.DB.Where("id = ?", botID).First(&botRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "机器人未找到"})
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description" binding:"required"`
		SystemPrompt string `json:"system_prompt" binding:"required"`
		Tools        string `json:"tools"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	skill := db.Skill{
		BotID:        botID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Enabled:      true,
	}
	if err := db.DB.Create(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	if instance := h.botManager.GetBot(botID); instance != nil {
		h.botManager.StopBot(botID)
		h.botManager.StartBot(botID)
	}

	c.JSON(http.StatusCreated, skill)
}

func (h *Handler) updateSkill(c *gin.Context) {
	botID := c.Param("id")
	skillID := c.Param("skillId")

	var skill db.Skill
	if err := db.DB.Where("id = ? AND bot_id = ?", skillID, botID).First(&skill).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能未找到"})
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		SystemPrompt *string `json:"system_prompt"`
		Tools        *string `json:"tools"`
		Enabled      *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if req.Tools != nil {
		updates["tools"] = *req.Tools
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := db.DB.Model(&skill).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	if instance := h.botManager.GetBot(botID); instance != nil {
		h.botManager.StopBot(botID)
		h.botManager.StartBot(botID)
	}

	db.DB.Where("id = ?", skillID).First(&skill)
	c.JSON(http.StatusOK, skill)
}

func (h *Handler) deleteSkill(c *gin.Context) {
	botID := c.Param("id")
	skillID := c.Param("skillId")

	var skill db.Skill
	if err := db.DB.Where("id = ? AND bot_id = ?", skillID, botID).First(&skill).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能未找到"})
		return
	}

	db.DB.Delete(&skill)

	if instance := h.botManager.GetBot(botID); instance != nil {
		h.botManager.StopBot(botID)
		h.botManager.StartBot(botID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) listKnowledgeBases(c *gin.Context) {
	var kbs []db.KnowledgeBase
	db.DB.Order("created_at DESC").Find(&kbs)
	c.JSON(http.StatusOK, kbs)
}

func (h *Handler) createKnowledgeBase(c *gin.Context) {
	var req struct {
		ID                  string `json:"id" binding:"required"`
		Name                string `json:"name" binding:"required"`
		Description         string `json:"description"`
		EmbeddingProviderID string `json:"embedding_provider_id" binding:"required"`
		EmbeddingModel      string `json:"embedding_model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	kb := db.KnowledgeBase{
		ID:                  req.ID,
		Name:                req.Name,
		Description:         req.Description,
		EmbeddingProviderID: req.EmbeddingProviderID,
		EmbeddingModel:      req.EmbeddingModel,
	}
	if err := db.DB.Create(&kb).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, kb)
}

func (h *Handler) getKnowledgeBase(c *gin.Context) {
	id := c.Param("id")
	var kb db.KnowledgeBase
	if err := db.DB.Where("id = ?", id).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库未找到"})
		return
	}
	c.JSON(http.StatusOK, kb)
}

func (h *Handler) updateKnowledgeBase(c *gin.Context) {
	id := c.Param("id")
	var kb db.KnowledgeBase
	if err := db.DB.Where("id = ?", id).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库未找到"})
		return
	}

	var req struct {
		Name                *string `json:"name"`
		Description         *string `json:"description"`
		EmbeddingProviderID *string `json:"embedding_provider_id"`
		EmbeddingModel      *string `json:"embedding_model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.EmbeddingProviderID != nil {
		updates["embedding_provider_id"] = *req.EmbeddingProviderID
	}
	if req.EmbeddingModel != nil {
		updates["embedding_model"] = *req.EmbeddingModel
	}
	db.DB.Model(&kb).Updates(updates)
	db.DB.Where("id = ?", id).First(&kb)
	c.JSON(http.StatusOK, kb)
}

func (h *Handler) deleteKnowledgeBase(c *gin.Context) {
	id := c.Param("id")
	var kb db.KnowledgeBase
	if err := db.DB.Where("id = ?", id).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库未找到"})
		return
	}

	var bindingCount int64
	db.DB.Model(&db.BotKnowledgeBinding{}).Where("knowledge_base_id = ?", id).Count(&bindingCount)
	if bindingCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("有 %d 个机器人绑定此知识库，请先解绑", bindingCount)})
		return
	}

	if h.weaviateClient != nil {
		if err := h.weaviateClient.DeleteByKBID(context.Background(), id); err != nil {
			log.Printf("[Knowledge] 删除 Weaviate 数据失败: %v", err)
		}
	}

	db.DB.Where("knowledge_base_id = ?", id).Delete(&db.ImportTask{})
	db.DB.Where("knowledge_base_id = ?", id).Delete(&db.BotKnowledgeBinding{})
	db.DB.Delete(&kb)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handler) uploadFile(c *gin.Context) {
	id := c.Param("id")
	var kb db.KnowledgeBase
	if err := db.DB.Where("id = ?", id).First(&kb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库未找到"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传文件"})
		return
	}
	defer file.Close()

	if h.importManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导入管理器未初始化"})
		return
	}

	task, err := h.importManager.ReceiveFile(id, header.Filename, file, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"task_id": task.ID,
		"message": "文件上传成功，后台导入中",
	})
}

func (h *Handler) listImportTasks(c *gin.Context) {
	kbID := c.Param("id")
	var tasks []db.ImportTask
	db.DB.Where("knowledge_base_id = ?", kbID).Order("created_at DESC").Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) getImportTask(c *gin.Context) {
	taskID := c.Param("id")
	var task db.ImportTask
	if err := db.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "导入任务未找到"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) getBotBindings(c *gin.Context) {
	botID := c.Param("id")
	var bindings []db.BotKnowledgeBinding
	db.DB.Where("bot_id = ?", botID).Find(&bindings)
	c.JSON(http.StatusOK, bindings)
}

func (h *Handler) updateBotBindings(c *gin.Context) {
	botID := c.Param("id")

	var req struct {
		KBIDs []string `json:"kb_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	tx := db.DB.Begin()
	tx.Where("bot_id = ?", botID).Delete(&db.BotKnowledgeBinding{})
	for _, kbID := range req.KBIDs {
		tx.Create(&db.BotKnowledgeBinding{
			BotID:           botID,
			KnowledgeBaseID: kbID,
		})
	}
	tx.Commit()

	if instance := h.botManager.GetBot(botID); instance != nil {
		h.botManager.StopBot(botID)
		h.botManager.StartBot(botID)
	}

	var bindings []db.BotKnowledgeBinding
	db.DB.Where("bot_id = ?", botID).Find(&bindings)
	c.JSON(http.StatusOK, bindings)
}
