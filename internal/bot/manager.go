// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hchw/bots-nest/internal/agent"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/llm"
)

type SessionManager struct {
	botID string
}

func NewSessionManager(botID string) *SessionManager {
	return &SessionManager{botID: botID}
}

func (m *SessionManager) GetOrCreate(userID, userName, convType, groupID string) (*db.Session, error) {
	key := fmt.Sprintf("%s:%s:%s", m.botID, userID, convType)

	var session db.Session
	result := db.DB.Where("session_key = ?", key).First(&session)
	if result.Error == nil {
		return &session, nil
	}

	session = db.Session{
		SessionKey:       key,
		BotID:            m.botID,
		UserID:           userID,
		UserName:         userName,
		ConversationType: convType,
		GroupID:          groupID,
	}
	if err := db.DB.Create(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (m *SessionManager) AddMessage(sessionKey, role, content string, tokens int) error {
	msg := db.Message{
		SessionKey: sessionKey,
		Role:       role,
		Content:    content,
		Tokens:     tokens,
	}
	return db.DB.Create(&msg).Error
}

func (m *SessionManager) GetHistory(sessionKey string, limit int) ([]db.Message, error) {
	var messages []db.Message
	result := db.DB.Where("session_key = ? AND expired = 0", sessionKey).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages)
	return messages, result.Error
}

func (m *SessionManager) GetSessions(botID string, page, pageSize int) ([]db.Session, int64, error) {
	var sessions []db.Session
	var total int64
	db.DB.Model(&db.Session{}).Where("bot_id = ?", botID).Count(&total)
	result := db.DB.Where("bot_id = ?", botID).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("updated_at DESC").
		Find(&sessions)
	return sessions, total, result.Error
}

func (m *SessionManager) ClearSession(sessionKey string) error {
	return db.DB.Model(&db.Message{}).
		Where("session_key = ?", sessionKey).
		Update("expired", true).Error
}

func (m *SessionManager) DeleteSession(sessionKey string) error {
	tx := db.DB.Begin()
	if err := tx.Where("session_key = ?", sessionKey).Delete(&db.Message{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("session_key = ?", sessionKey).Delete(&db.Session{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (m *SessionManager) Compress(sessionKey string, summary string) error {
	tx := db.DB.Begin()
	if err := tx.Model(&db.Message{}).
		Where("session_key = ? AND expired = 0", sessionKey).
		Update("expired", true).Error; err != nil {
		tx.Rollback()
		return err
	}
	msg := db.Message{
		SessionKey: sessionKey,
		Role:       "system",
		Content:    "对话摘要: " + summary,
		Tokens:     len(summary) / 4,
	}
	if err := tx.Create(&msg).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (m *SessionManager) TotalTokens(sessionKey string) (int, error) {
	var total int
	result := db.DB.Model(&db.Message{}).
		Select("COALESCE(SUM(tokens), 0)").
		Where("session_key = ? AND expired = 0", sessionKey).
		Scan(&total)
	return total, result.Error
}

type BotManager struct {
	mu   sync.Mutex
	bots map[string]*BotInstance
}

func NewBotManager() *BotManager {
	return &BotManager{
		bots: make(map[string]*BotInstance),
	}
}

func (m *BotManager) AddBot(botID string, bot *BotInstance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bots[botID] = bot
}

func (m *BotManager) GetBot(botID string) *BotInstance {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bots[botID]
}

func (m *BotManager) RemoveBot(botID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.bots, botID)
}

func (m *BotManager) StartBot(botID string) error {
	if m.GetBot(botID) != nil {
		return fmt.Errorf("机器人 %s 已在运行", botID)
	}

	var b db.Bot
	if err := db.DB.Where("id = ?", botID).First(&b).Error; err != nil {
		return fmt.Errorf("机器人 %s 未找到", botID)
	}

	wecom := NewWeComClient(b.WecomBotID, b.WecomSecret)
	skillEng := NewSkillEngine()

	var skills []db.Skill
	db.DB.Where("bot_id = ? AND enabled = 1", b.ID).Find(&skills)
	var botSkills []Skill
	for _, s := range skills {
		botSkills = append(botSkills, Skill{
			Name:         s.Name,
			Description:  s.Description,
			SystemPrompt: s.SystemPrompt,
			Tools:        s.Tools,
			Enabled:      s.Enabled,
		})
	}
	skillEng.Register(b.ID, botSkills)

	lm := 4096
	if b.MaxSessionTokens != 0 {
		lm = b.MaxSessionTokens
	}
	temp := 0.0
	if b.LLMTemperature != nil {
		temp = *b.LLMTemperature
	}
	maxTokens := 0
	if b.LLMMaxTokens != nil {
		maxTokens = *b.LLMMaxTokens
	}
	instance := &BotInstance{
		ID: b.ID,
		Config: BotConfig{
			ID:               b.ID,
			Name:             b.Name,
			WecomBotID:       b.WecomBotID,
			WecomSecret:      b.WecomSecret,
			LLMProviderID:    b.LLMProviderID,
			LLMModel:         b.LLMModel,
			LLMTemperature:   temp,
			LLMMaxTokens:     maxTokens,
			MaxSessionTokens: lm,
			Enabled:          b.Enabled,
		},
		WeCom:      wecom,
		SkillEng:   skillEng,
		SessionMgr: NewSessionManager(b.ID),
	}

	wecom.SetMessageHandler(func(msg *WeComMessage) {
		if err := instance.processWeComMessage(msg); err != nil {
			log.Printf("机器人 %s 消息处理失败: %v", b.ID, err)
		}
	})

	wecom.SetStatusCallback(func(status string) {
		db.DB.Model(&db.Bot{}).Where("id = ?", b.ID).Update("status", status)
	})

	log.Printf("[管理器] 启动机器人 %s (wecom_bot_id=%s)", b.ID, b.WecomBotID)

	if err := wecom.Connect(); err != nil {
		log.Printf("[管理器] 机器人 %s 连接失败: %v", b.ID, err)
		return fmt.Errorf("机器人 %s 连接失败: %w", b.ID, err)
	}

	m.AddBot(b.ID, instance)
	log.Printf("[管理器] 机器人 %s 启动成功", b.ID)
	return nil
}

func (m *BotManager) StopBot(botID string) error {
	m.mu.Lock()
	instance := m.bots[botID]
	if instance == nil {
		m.mu.Unlock()
		log.Printf("[管理器] 停止机器人 %s 失败: 未在运行", botID)
		return fmt.Errorf("机器人 %s 未在运行", botID)
	}
	delete(m.bots, botID)
	m.mu.Unlock()

	log.Printf("[管理器] 正在停止机器人 %s", botID)
	instance.WeCom.Close()
	log.Printf("[管理器] 机器人 %s 已停止", botID)
	return nil
}

func (m *BotManager) GetAll() map[string]*BotInstance {
	return m.bots
}

func (m *BotManager) LoadFromDB() {
	var bots []db.Bot
	db.DB.Where("enabled = 1").Find(&bots)

	for _, b := range bots {
		if err := m.StartBot(b.ID); err != nil {
			log.Printf("启动机器人 %s 失败: %v", b.Name, err)
		}
	}
}

func (b *BotInstance) processWeComMessage(msg *WeComMessage) error {
	body := msg.Body

	userID := ""
	if body.From != nil {
		userID = body.From.UserID
	}

	convType := "single"
	groupID := ""
	if body.ChatID != "" {
		convType = "group"
		groupID = body.ChatID
	} else if body.ChatType == "group" {
		convType = "group"
	}

	session, err := b.SessionMgr.GetOrCreate(userID, userID, convType, groupID)
	if err != nil {
		return fmt.Errorf("获取/创建会话失败: %w", err)
	}

	userContent := ""
	switch body.MsgType {
	case "text":
		if body.Text != nil {
			userContent = body.Text.Content
		}
	default:
		log.Printf("机器人 %s 收到不支持的消息类型: %s", b.ID, body.MsgType)
		return nil
	}

	if userContent == "" {
		log.Printf("机器人 %s 收到空消息, 跳过处理", b.ID)
		return nil
	}

	if err := b.SessionMgr.AddMessage(session.SessionKey, "user", userContent, llm.EstimateTokens(userContent)); err != nil {
		return fmt.Errorf("存储用户消息失败: %w", err)
	}

	// Build LLM messages
	var msgs []llm.ChatMessage

	// Inject matched skill system prompt
	var matchedSkill *Skill
	if skill := b.SkillEng.Match(b.ID, userContent); skill != nil {
		matchedSkill = skill
		msgs = append(msgs, llm.ChatMessage{Role: "system", Content: skill.SystemPrompt})
	}

	// Load history (newest first), reverse to chronological order
	history, err := b.SessionMgr.GetHistory(session.SessionKey, 20)
	if err != nil {
		return fmt.Errorf("加载历史失败: %w", err)
	}
	for i := len(history) - 1; i >= 0; i-- {
		msgs = append(msgs, llm.ChatMessage{
			Role:    history[i].Role,
			Content: history[i].Content,
		})
	}

	// Build tools list from MCP + Skills + Shell Agent
	tools := b.buildTools(matchedSkill)

	// Create LLM client from bot's provider
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", b.Config.LLMProviderID).First(&provider).Error; err != nil {
		return fmt.Errorf("LLM Provider %s 未找到: %w", b.Config.LLMProviderID, err)
	}
	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, b.Config.LLMModel)
	if b.Config.LLMTemperature != 0 {
		llmClient.Temperature = b.Config.LLMTemperature
	}
	if b.Config.LLMMaxTokens != 0 {
		llmClient.MaxTokens = b.Config.LLMMaxTokens
	}

	// Tool call loop
	shellExec := agent.NewShellExecutor(nil, 30*time.Second, 4096)

	var finalReply string
	maxIterations := 10

	for iter := 0; iter < maxIterations; iter++ {
		resp, err := llmClient.Chat(msgs, tools)
		if err != nil {
			return fmt.Errorf("LLM 调用失败: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			finalReply = resp.Content
			break
		}

		// Add assistant message with tool calls to context
		msgs = append(msgs, llm.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result := b.executeToolCall(tc, shellExec)
			msgs = append(msgs, llm.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalReply == "" {
		finalReply = "抱歉，处理超时，请重试"
	}

	// Store assistant reply
	if err := b.SessionMgr.AddMessage(session.SessionKey, "assistant", finalReply, llm.EstimateTokens(finalReply)); err != nil {
		return fmt.Errorf("存储回复失败: %w", err)
	}

	return b.WeCom.SendReply(msg.Headers.ReqID, finalReply)
}

func (b *BotInstance) buildTools(matchedSkill *Skill) []llm.ToolDefinition {
	var tools []llm.ToolDefinition

	// Load enabled MCPs and merge their tools (prefixed with mcp ID)
	var mcps []db.MCP
	db.DB.Where("enabled = 1").Find(&mcps)

	for _, mcp := range mcps {
		var mcpTools []struct {
			Type     string          `json:"type"`
			Function llm.FunctionDef `json:"function"`
		}
		if err := json.Unmarshal([]byte(mcp.Tools), &mcpTools); err != nil || len(mcpTools) == 0 {
			continue
		}
		for _, t := range mcpTools {
			prefixedName := mcp.ID + "__" + t.Function.Name
			tools = append(tools, llm.ToolDefinition{
				Type: t.Type,
				Function: llm.FunctionDef{
					Name:        prefixedName,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			})
		}
	}

	// Inject matched skill's tools
	if matchedSkill != nil && matchedSkill.Tools != "" {
		var skillTools []llm.ToolDefinition
		if err := json.Unmarshal([]byte(matchedSkill.Tools), &skillTools); err == nil {
			tools = append(tools, skillTools...)
		}
	}

	// Add Shell Agent tool
	tools = append(tools, llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "shell_exec",
			Description: "在服务器上执行 Shell 命令并返回输出结果",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "要执行的 Shell 命令",
					},
				},
				"required": []string{"command"},
			},
		},
	})

	return tools
}

func (b *BotInstance) executeToolCall(tc llm.ToolCall, shellExec *agent.ShellExecutor) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "参数解析失败: " + err.Error()
	}

	switch {
	case tc.Function.Name == "shell_exec":
		cmd, _ := args["command"].(string)
		if cmd == "" {
			return "缺少 command 参数"
		}
		result, err := shellExec.Execute(cmd)
		if err != nil {
			return "Shell 执行失败: " + err.Error()
		}
		return result

	default:
		// MCP tool format: {mcpID}__{toolName}
		parts := strings.SplitN(tc.Function.Name, "__", 2)
		if len(parts) != 2 {
			return "未知工具: " + tc.Function.Name
		}
		mcpID := parts[0]
		toolName := parts[1]

		var mcp db.MCP
		if err := db.DB.Where("id = ? AND enabled = 1", mcpID).First(&mcp).Error; err != nil {
			return "MCP 服务器未找到: " + mcpID
		}

		client := agent.NewMCPClient(mcp.Name, mcp.Endpoint)
		result, err := client.Call(toolName, args)
		if err != nil {
			return "MCP 调用失败: " + err.Error()
		}
		return result
	}
}
