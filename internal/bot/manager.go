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
	"github.com/hchw/bots-nest/internal/config"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/llm"
	"github.com/hchw/bots-nest/internal/skilltool"
)

const DefaultSystemPrompt = "你是智能助手。"

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
	cfg  *config.Config
}

func NewBotManager(cfg *config.Config) *BotManager {
	return &BotManager{
		bots: make(map[string]*BotInstance),
		cfg:  cfg,
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

	builtinSkills, err := LoadBuiltinSkills(m.cfg.SkillsDir)
	if err != nil {
		log.Printf("[管理器] 机器人 %s 加载内置 Skill 失败: %v", b.ID, err)
	}
	skillMap := make(map[string]Skill)
	for _, s := range builtinSkills {
		skillMap[s.Name] = s
	}

	var dbSkills []db.Skill
	db.DB.Where("bot_id = ? AND enabled = 1", b.ID).Find(&dbSkills)
	for _, s := range dbSkills {
		skillMap[s.Name] = Skill{
			Name:         s.Name,
			Description:  s.Description,
			SystemPrompt: s.SystemPrompt,
			Tools:        s.Tools,
			Enabled:      s.Enabled,
		}
	}

	var botSkills []Skill
	for _, s := range skillMap {
		botSkills = append(botSkills, s)
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
			GoJudgeEndpoint:  m.cfg.GoJudgeEndpoint,
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

	// Handle special commands
	switch cmd := strings.TrimSpace(userContent); cmd {
	case "/clear":
		if err := b.SessionMgr.ClearSession(session.SessionKey); err != nil {
			return fmt.Errorf("清空会话失败: %w", err)
		}
		b.WeCom.SendReply(msg.Headers.ReqID, "会话已清空")
		return nil
	case "/compress":
		return b.handleCompressCommand(msg, session.SessionKey)
	}

	// Build LLM messages
	var msgs []llm.ChatMessage

	msgs = append(msgs, llm.ChatMessage{Role: "system", Content: DefaultSystemPrompt})

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
	tools := b.buildTools()

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

	// Tool call loop with streaming
	shellExec := agent.NewShellExecutor(nil, 30*time.Second, 4096)
	reqID := msg.Headers.ReqID
	streamID := fmt.Sprintf("s_%d", time.Now().UnixNano())

	var finalReply string
	maxIterations := 10

	var displayBuf strings.Builder
	var lastFlushedLen int
	var lastFlush time.Time

	flushDisplay := func(finish bool) {
		content := displayBuf.String()
		if len(content) > 0 || finish {
			b.WeCom.SendStreamChunk(reqID, streamID, content, finish)
			lastFlush = time.Now()
			lastFlushedLen = displayBuf.Len()
		}
	}

	writeDisplay := func(content string) {
		displayBuf.WriteString(content)
		if displayBuf.Len()-lastFlushedLen >= 200 || time.Since(lastFlush) > time.Second {
			flushDisplay(false)
		}
	}

	writeDisplay("🤔 思考中...\n")

	for iter := 0; iter < maxIterations; iter++ {
		eventCh, err := llmClient.ChatStream(msgs, tools)
		if err != nil {
			writeDisplay("\n处理出错: " + err.Error())
			flushDisplay(true)
			return fmt.Errorf("LLM 流式调用失败: %w", err)
		}

		var contentBuf strings.Builder
		var toolCalls []llm.ToolCall

		for event := range eventCh {
			if event.Content != "" {
				contentBuf.WriteString(event.Content)
				writeDisplay(event.Content)
			}
			if len(event.ToolCalls) > 0 {
				toolCalls = event.ToolCalls
			}
			if event.Done {
				break
			}
		}

		if len(toolCalls) > 0 {
			flushDisplay(false)
		}

		if len(toolCalls) == 0 {
			finalReply = contentBuf.String()
			break
		}

		// Add assistant message with tool calls to context
		msgs = append(msgs, llm.ChatMessage{
			Role:      "assistant",
			Content:   contentBuf.String(),
			ToolCalls: toolCalls,
		})

		// Execute each tool call, streaming output to user
		for _, tc := range toolCalls {
			var result string
			if tc.Function.Name == "activate_skill" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					result = "参数解析失败: " + err.Error()
				} else {
					skillName, _ := args["skill_name"].(string)
					if skill := b.SkillEng.Lookup(b.ID, skillName); skill != nil {
						result = skill.SystemPrompt
						if skill.Tools != "" {
							var skillTools []llm.ToolDefinition
							if err := json.Unmarshal([]byte(skill.Tools), &skillTools); err == nil {
								tools = append(tools, skillTools...)
							}
						}
						loadGoJudgeTools(b.ID, skill, &tools)
					} else {
						result = "未找到技能: " + skillName
					}
				}
			} else {
				result = b.executeToolCall(tc, shellExec, func(chunk string) {
					writeDisplay(chunk)
				})
			}
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

	// Send final finish signal (flushes remaining content if any)
	flushDisplay(true)

	// Store assistant reply
	if err := b.SessionMgr.AddMessage(session.SessionKey, "assistant", finalReply, llm.EstimateTokens(finalReply)); err != nil {
		return fmt.Errorf("存储回复失败: %w", err)
	}

	// Auto-compress if token limit exceeded
	b.autoCompressIfNeeded(session.SessionKey, reqID)

	return nil
}

func (b *BotInstance) buildTools() []llm.ToolDefinition {
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

	// Register all enabled skills as callable activate_skill tool
	skills := b.SkillEng.GetSkills(b.ID)
	skillNames := make([]string, 0, len(skills))
	for _, s := range skills {
		if s.Enabled {
			skillNames = append(skillNames, s.Name)
		}
	}
	if len(skillNames) > 0 {
		var desc strings.Builder
		desc.WriteString("根据用户的问题唤起最合适的技能来获取针对性的回复指令。可选技能:\n")
		for _, s := range skills {
			if s.Enabled {
				desc.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
			}
		}
		tools = append(tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "activate_skill",
				Description: desc.String(),
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"skill_name": map[string]interface{}{
							"type": "string",
							"enum": skillNames,
						},
					},
					"required": []string{"skill_name"},
				},
			},
		})
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

func (b *BotInstance) handleCompressCommand(msg *WeComMessage, sessionKey string) error {
	history, err := b.SessionMgr.GetHistory(sessionKey, 100)
	if err != nil {
		return fmt.Errorf("获取历史消息失败: %w", err)
	}
	if len(history) == 0 {
		b.WeCom.SendReply(msg.Headers.ReqID, "没有可压缩的消息")
		return nil
	}

	var convBuf strings.Builder
	for i := len(history) - 1; i >= 0; i-- {
		convBuf.WriteString(history[i].Role + ": " + history[i].Content + "\n")
	}

	summaryMsgs := []llm.ChatMessage{
		{Role: "system", Content: "你是一个对话摘要助手。请用中文简洁地总结以下对话的核心内容和关键信息，保留重要细节。"},
		{Role: "user", Content: "请总结以下对话：\n\n" + convBuf.String()},
	}

	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", b.Config.LLMProviderID).First(&provider).Error; err != nil {
		return fmt.Errorf("LLM Provider %s 未找到: %w", b.Config.LLMProviderID, err)
	}
	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, b.Config.LLMModel)
	llmClient.Temperature = 0.3

	resp, err := llmClient.Chat(summaryMsgs, nil)
	if err != nil {
		return fmt.Errorf("LLM 摘要生成失败: %w", err)
	}

	if err := b.SessionMgr.Compress(sessionKey, resp.Content); err != nil {
		return fmt.Errorf("保存压缩摘要失败: %w", err)
	}

	b.WeCom.SendReply(msg.Headers.ReqID, "已压缩会话")
	return nil
}

func (b *BotInstance) autoCompressIfNeeded(sessionKey string, reqID string) {
	totalTokens, err := b.SessionMgr.TotalTokens(sessionKey)
	if err != nil || totalTokens <= b.Config.MaxSessionTokens {
		return
	}

	history, err := b.SessionMgr.GetHistory(sessionKey, 100)
	if err != nil || len(history) == 0 {
		return
	}

	var convBuf strings.Builder
	for i := len(history) - 1; i >= 0; i-- {
		convBuf.WriteString(history[i].Role + ": " + history[i].Content + "\n")
	}

	summaryMsgs := []llm.ChatMessage{
		{Role: "system", Content: "你是一个对话摘要助手。请用中文简洁地总结以下对话的核心内容和关键信息，保留重要细节。"},
		{Role: "user", Content: "请总结以下对话：\n\n" + convBuf.String()},
	}

	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", b.Config.LLMProviderID).First(&provider).Error; err != nil {
		log.Printf("自动压缩失败: 获取 LLM Provider 出错: %v", err)
		return
	}
	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, b.Config.LLMModel)
	llmClient.Temperature = 0.3

	resp, err := llmClient.Chat(summaryMsgs, nil)
	if err != nil {
		log.Printf("自动压缩失败: LLM 摘要生成出错: %v", err)
		return
	}

	if err := b.SessionMgr.Compress(sessionKey, resp.Content); err != nil {
		log.Printf("自动压缩失败: 保存摘要出错: %v", err)
		return
	}

	log.Printf("机器人 %s 会话 %s 自动压缩完成 (tokens: %d > %d)", b.ID, sessionKey, totalTokens, b.Config.MaxSessionTokens)
	notifyContent := fmt.Sprintf("✅ 会话已自动压缩\n\n📝 摘要:\n%s", resp.Content)
	if err := b.WeCom.SendReply(reqID, notifyContent); err != nil {
		log.Printf("自动压缩通知发送失败: %v", err)
	}
}

func loadGoJudgeTools(botID string, skill *Skill, tools *[]llm.ToolDefinition) {
	var dbSkill db.Skill
	if err := db.DB.Where("bot_id = ? AND name = ?", botID, skill.Name).First(&dbSkill).Error; err != nil {
		log.Printf("[go-judge] 未找到对应 Skill: %s", skill.Name)
		return
	}
	goJudgeTools, err := skilltool.ListToolsBySkill(botID, dbSkill.ID)
	if err != nil {
		log.Printf("[go-judge] 加载 Tool 失败: %v", err)
		return
	}
	for _, t := range goJudgeTools {
		fnName := "gjtool__" + fmt.Sprintf("%d", t.ID)
		desc := t.Name
		if t.Prompt != "" {
			desc += ": " + t.Prompt
		}
		desc += fmt.Sprintf(" (语言: %s)", t.Language)
		*tools = append(*tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        fnName,
				Description: desc,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				},
			},
		})
	}
}

func (b *BotInstance) executeToolCall(tc llm.ToolCall, shellExec *agent.ShellExecutor, streamFn func(string)) string {
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

		header := fmt.Sprintf("\n\n🖥️ 执行: `%s`\n```\n", cmd)
		streamFn(header)

		lineCh, err := shellExec.ExecuteStream(cmd)
		if err != nil {
			errMsg := fmt.Sprintf("```\nShell 执行失败: %s\n", err.Error())
			streamFn(errMsg)
			return errMsg
		}

		var result strings.Builder
		for line := range lineCh {
			result.WriteString(line)
			streamFn(line)
		}

		footer := "```\n"
		streamFn(footer)

		return result.String()

	case strings.HasPrefix(tc.Function.Name, "gjtool__"):
		toolIDStr := strings.TrimPrefix(tc.Function.Name, "gjtool__")
		var toolID uint
		if _, err := fmt.Sscanf(toolIDStr, "%d", &toolID); err != nil {
			return "无效的 Tool ID: " + toolIDStr
		}

		var gjt db.GoJudgeTool
		if err := db.DB.First(&gjt, toolID).Error; err != nil {
			return "go-judge Tool 未找到"
		}

		streamFn(fmt.Sprintf("\n\n🛠️ 执行 Tool: %s (%s)\n", gjt.Name, gjt.Language))

		executor := skilltool.NewExecutor(b.Config.GoJudgeEndpoint)
		resp, err := executor.Execute(&skilltool.ExecuteRequest{
			Lang: gjt.Language,
			Src:  gjt.Code,
		})
		if err != nil {
			return "go-judge 执行失败: " + err.Error()
		}

		var buf strings.Builder
		if resp.Stdout != "" {
			buf.WriteString("stdout:\n" + resp.Stdout + "\n")
		}
		if resp.Stderr != "" {
			buf.WriteString("stderr:\n" + resp.Stderr + "\n")
		}
		buf.WriteString(fmt.Sprintf("status: %d", resp.Status))
		if resp.Error != "" {
			buf.WriteString("\nerror: " + resp.Error)
		}
		return buf.String()

	default:
		// MCP tool format: {mcpID}__{toolName}
		parts := strings.SplitN(tc.Function.Name, "__", 2)
		if len(parts) != 2 {
			return "未知工具: " + tc.Function.Name
		}
		mcpID := parts[0]
		toolName := parts[1]

		streamFn(fmt.Sprintf("\n\n🛠️ 调用 MCP 工具: %s\n", toolName))

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
