// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"github.com/hchw/bots-nest/internal/bot"
	"github.com/hchw/bots-nest/internal/db"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type Handler struct {
	botManager *bot.BotManager
}

func NewHandler(bm *bot.BotManager) *Handler {
	return &Handler{botManager: bm}
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
	var skills []db.Skill
	db.DB.Where("bot_id = ?", botID).Find(&skills)
	c.JSON(http.StatusOK, skills)
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

func autoDiscoverTools(endpoint string) string {
	client := resty.New()
	resp, err := client.R().Get(endpoint + "/tools")
	if err != nil {
		resp, err = client.R().Get(endpoint)
		if err != nil {
			return "[]"
		}
	}

	var tools interface{}
	if err := json.Unmarshal(resp.Body(), &tools); err != nil {
		return "[]"
	}
	data, _ := json.Marshal(tools)
	return string(data)
}

func (h *Handler) createMCP(c *gin.Context) {
	var req struct {
		ID       string `json:"id" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	endpoint := strings.TrimRight(req.Endpoint, "/")
	tools := autoDiscoverTools(endpoint)
	mcp := db.MCP{
		ID:       req.ID,
		Name:     req.Name,
		Endpoint: endpoint,
		Tools:    tools,
		Enabled:  true,
	}
	if err := db.DB.Create(&mcp).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mcp)
}

func (h *Handler) updateMCP(c *gin.Context) {
	id := c.Param("id")
	var mcp db.MCP
	if err := db.DB.Where("id = ?", id).First(&mcp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
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
		endpoint := strings.TrimRight(req.Endpoint, "/")
		updates["endpoint"] = endpoint
		updates["tools"] = autoDiscoverTools(endpoint)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := db.DB.Model(&mcp).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	db.DB.Where("id = ?", id).First(&mcp)
	c.JSON(http.StatusOK, mcp)
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
