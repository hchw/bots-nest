// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hchw/bots-nest/internal/agent"
	"github.com/hchw/bots-nest/internal/config"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/knowledge"
	"github.com/hchw/bots-nest/internal/llm"
	"github.com/hchw/bots-nest/internal/skilltool"
	"github.com/hchw/bots-nest/internal/task"
)

const DefaultSystemPrompt = "你是智能助手。当有可用工具时，必须通过调用工具来完成任务，不要编造执行结果。"

type SessionManager struct {
	botID    string
	platform string
}

func NewSessionManager(botID string) *SessionManager {
	return &SessionManager{botID: botID}
}

func (m *SessionManager) GetOrCreate(userID, userName, convType, groupID string) (*db.Session, error) {
	key := m.sessionKey(userID, convType)

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

func (m *SessionManager) sessionKey(userID, convType string) string {
	if m.platform != "" {
		return fmt.Sprintf("%s:%s:%s:%s", m.botID, m.platform, userID, convType)
	}
	return fmt.Sprintf("%s:%s:%s", m.botID, userID, convType)
}

func (m *SessionManager) GetOrCreateWithKey(sessionKey, userID, userName, convType, groupID string) (*db.Session, error) {
	var session db.Session
	result := db.DB.Where("session_key = ?", sessionKey).First(&session)
	if result.Error == nil {
		return &session, nil
	}

	session = db.Session{
		SessionKey:       sessionKey,
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
	mu              sync.Mutex
	bots            map[string]*BotInstance
	cfg             *config.Config
	weaviateClient  *knowledge.WeaviateClient
	builtinEmbedder *knowledge.BuiltinEmbedder
	taskEngine      *task.Engine
}

func NewBotManager(cfg *config.Config, wc *knowledge.WeaviateClient, be *knowledge.BuiltinEmbedder) *BotManager {
	return &BotManager{
		bots:            make(map[string]*BotInstance),
		cfg:             cfg,
		weaviateClient:  wc,
		builtinEmbedder: be,
	}
}

func (m *BotManager) SetTaskEngine(engine *task.Engine) {
	m.taskEngine = engine
}

func (m *BotManager) TaskEngine() *task.Engine {
	return m.taskEngine
}

func (m *BotManager) SetupTaskExecutor() {
	if m.taskEngine == nil {
		return
	}

	executor := task.NewExecutor(func(ctx context.Context, params task.ExecuteParams) error {
		m.mu.Lock()
		instance, ok := m.bots[params.BotID]
		m.mu.Unlock()
		if !ok || instance == nil {
			return fmt.Errorf("机器人 %s 未运行", params.BotID)
		}

		log.Printf("[任务执行] bot=%s session=%s route=%s task=%s", params.BotID, params.SessionKey, params.Route, params.TaskID)

		if params.TaskType == "session" {
			return m.executeTaskForSession(instance, params)
		}

		return m.executeTaskForBot(instance, params)
	})
	m.taskEngine.SetExecutor(executor)
}

func (m *BotManager) executeTaskForSession(instance *BotInstance, params task.ExecuteParams) error {
	reqID := fmt.Sprintf("task_%s", strings.ReplaceAll(params.TaskID, "-", ""))
	sessionKey := params.SessionKey

	var session db.Session
	if err := db.DB.Where("session_key = ?", sessionKey).First(&session).Error; err != nil {
		return fmt.Errorf("查询会话 %s 失败: %w", sessionKey, err)
	}

	chatID, chatType := sessionChatInfo(&session)
	return m.executeWithRoute(instance, reqID, sessionKey, params.Route, params.Content, chatID, chatType)
}

func (m *BotManager) executeTaskForBot(instance *BotInstance, params task.ExecuteParams) error {
	var sessions []db.Session
	if err := db.DB.Where("bot_id = ?", params.BotID).Find(&sessions).Error; err != nil {
		return fmt.Errorf("查询机器人 %s 的会话失败: %w", params.BotID, err)
	}

	if len(sessions) == 0 {
		log.Printf("[任务执行] 机器人 %s 没有活跃会话", params.BotID)
		return nil
	}

	prefix := ""
	if params.GlobalTaskName != "" {
		prefix = fmt.Sprintf("[%s] ", params.GlobalTaskName)
	}

	for _, s := range sessions {
		sessionKey := s.SessionKey
		reqID := fmt.Sprintf("task_%s_%s", strings.ReplaceAll(params.TaskID, "-", ""), sessionKey)

		var logEntry = task.TaskExecutionLog{
			ID:          uuid.New().String(),
			TaskID:      params.TaskID,
			TaskType:    "global",
			BotID:       params.BotID,
			SessionKey:  sessionKey,
			Status:      "running",
			TriggerType: "schedule",
			ExecutedAt:  time.Now(),
		}

		content := params.Content
		if prefix != "" {
			content = prefix + content
		}

		chatID, chatType := sessionChatInfo(&s)
		err := m.executeWithRoute(instance, reqID, sessionKey, params.Route, content, chatID, chatType)
		if err != nil {
			logEntry.Status = "failed"
			logEntry.Result = err.Error()
		} else {
			logEntry.Status = "success"
		}

		if err := m.taskEngine.Store().CreateExecutionLog(&logEntry); err != nil {
			log.Printf("[任务执行] 记录执行日志失败: %v", err)
		}
	}
	return nil
}

func (m *BotManager) executeWithRoute(instance *BotInstance, reqID, sessionKey, route, content string, chatID string, chatType int) error {
	if route == "direct" {
		return instance.Platform.SendActiveMsg(reqID, chatID, chatType, content)
	}

	if err := instance.Platform.SendActiveMsg(reqID, chatID, chatType, "⏰ 定时任务已开始执行..."); err != nil {
		log.Printf("[任务执行] 发送前置通知失败: %v", err)
	}

	provider, err := m.getLLMProvider(instance)
	if err != nil {
		errMsg := fmt.Sprintf("⏰ 定时任务执行失败: %v", err)
		instance.Platform.SendActiveMsg(reqID, chatID, chatType, errMsg)

		m.taskEngine.Store().CreateExecutionLog(&task.TaskExecutionLog{
			ID:          uuid.New().String(),
			TaskID:      reqID,
			TaskType:    "session",
			BotID:       instance.ID,
			SessionKey:  sessionKey,
			Status:      "failed",
			Result:      err.Error(),
			TriggerType: "schedule",
			ExecutedAt:  time.Now(),
		})
		return err
	}

	llmClient := llm.NewOpenAIClient(provider.Endpoint, provider.APIKey, instance.Config.LLMModel)
	if instance.Config.LLMTemperature != 0 {
		llmClient.Temperature = instance.Config.LLMTemperature
	}
	if instance.Config.LLMMaxTokens != 0 {
		llmClient.MaxTokens = instance.Config.LLMMaxTokens
	}

	history, err := instance.SessionMgr.GetHistory(sessionKey, 10)
	if err != nil {
		log.Printf("[任务执行] 获取会话历史失败: %v", err)
	}

	var msgs []llm.ChatMessage
	msgs = append(msgs, llm.ChatMessage{Role: "system", Content: DefaultSystemPrompt})
	if len(history) > 0 {
		for i := len(history) - 1; i >= 0; i-- {
			msgs = append(msgs, llm.ChatMessage{
				Role:    history[i].Role,
				Content: history[i].Content,
			})
		}
	}
	msgs = append(msgs, llm.ChatMessage{Role: "user", Content: content})

	tools := instance.buildTools()
	shellExec := agent.NewShellExecutor(nil, 30*time.Second, 4096)

	var finalReply string
	maxIterations := 10

	log.Printf("[任务执行] 开始 agent 循环, maxIterations=%d, msgs=%d, tools=%d", maxIterations, len(msgs), len(tools))

	for iter := 0; iter < maxIterations; iter++ {
		log.Printf("[任务执行] iter=%d/%d 调用 Chat msgs=%d tools=%d", iter+1, maxIterations, len(msgs), len(tools))
		resp, err := llmClient.Chat(msgs, tools)
		if err != nil {
			errMsg := fmt.Sprintf("⏰ 定时任务执行失败: %v", err)
			instance.Platform.SendActiveMsg(reqID, chatID, chatType, errMsg)

			m.taskEngine.Store().CreateExecutionLog(&task.TaskExecutionLog{
				ID:          uuid.New().String(),
				TaskID:      reqID,
				TaskType:    "session",
				BotID:       instance.ID,
				SessionKey:  sessionKey,
				Status:      "failed",
				Result:      err.Error(),
				TriggerType: "schedule",
				ExecutedAt:  time.Now(),
			})
			return err
		}

		if len(resp.ToolCalls) == 0 {
			finalReply = resp.Content
			log.Printf("[任务执行] iter=%d 无 tool 调用, content=%q, 结束循环", iter+1, finalReply)
			break
		}

		var toolNames []string
		for _, tc := range resp.ToolCalls {
			toolNames = append(toolNames, tc.Function.Name)
		}
		log.Printf("[任务执行] iter=%d 收到 tool 调用: %v", iter+1, toolNames)

		msgs = append(msgs, llm.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			var result string
			if tc.Function.Name == "activate_skill" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					result = "参数解析失败: " + err.Error()
				} else {
					skillName, _ := args["skill_name"].(string)
					if skill := instance.SkillEng.Lookup(instance.ID, skillName); skill != nil {
						result = skill.SystemPrompt
						if skill.Tools != "" {
							var skillTools []llm.ToolDefinition
							if err := json.Unmarshal([]byte(skill.Tools), &skillTools); err == nil {
								tools = append(tools, skillTools...)
							}
						}
						loadGoJudgeTools(instance.ID, skill, &tools)
					} else {
						result = "未找到技能: " + skillName
					}
				}
			} else {
				result = instance.executeToolCall(tc, shellExec, sessionKey, func(chunk string) {})
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

	if err := instance.SessionMgr.AddMessage(sessionKey, "assistant", finalReply, llm.EstimateTokens(finalReply)); err != nil {
		log.Printf("[任务执行] 存储回复失败: %v", err)
	}

	return instance.Platform.SendActiveMsg(reqID, chatID, chatType, finalReply)
}

func sessionChatInfo(s *db.Session) (chatID string, chatType int) {
	if s.ConversationType == "group" && s.GroupID != "" {
		return s.GroupID, 2
	}
	return s.UserID, 1
}

func (m *BotManager) getLLMProvider(instance *BotInstance) (*db.LLMProvider, error) {
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", instance.Config.LLMProviderID).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("LLM Provider %s 未找到: %w", instance.Config.LLMProviderID, err)
	}
	return &provider, nil
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

func (m *BotManager) createPlatformClient(b *db.Bot) (PlatformClient, error) {
	platformType := b.PlatformType
	if platformType == "" {
		platformType = "wecom"
	}

	switch platformType {
	case "wecom":
		var cfg struct {
			BotID  string `json:"bot_id"`
			Secret string `json:"secret"`
		}
		if b.PlatformConfig != "" {
			if err := json.Unmarshal([]byte(b.PlatformConfig), &cfg); err != nil {
				return nil, fmt.Errorf("解析 WeCom 配置失败: %w", err)
			}
		}
		if cfg.BotID == "" || cfg.Secret == "" {
			return nil, fmt.Errorf("WeCom 配置缺少 bot_id 或 secret")
		}
		return NewWeComClient(cfg.BotID, cfg.Secret), nil
	case "dingtalk":
		var cfg struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if b.PlatformConfig != "" {
			if err := json.Unmarshal([]byte(b.PlatformConfig), &cfg); err != nil {
				return nil, fmt.Errorf("解析钉钉配置失败: %w", err)
			}
		}
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, fmt.Errorf("钉钉配置缺少 client_id 或 client_secret")
		}
		return NewDingTalkClient(cfg.ClientID, cfg.ClientSecret), nil
	default:
		return nil, fmt.Errorf("不支持的平台类型: %s", platformType)
	}
}

func (m *BotManager) StartBot(botID string) error {
	if m.GetBot(botID) != nil {
		return fmt.Errorf("机器人 %s 已在运行", botID)
	}

	var b db.Bot
	if err := db.DB.Where("id = ?", botID).First(&b).Error; err != nil {
		return fmt.Errorf("机器人 %s 未找到", botID)
	}

	platformClient, err := m.createPlatformClient(&b)
	if err != nil {
		return fmt.Errorf("创建平台客户端失败: %w", err)
	}

	skillEng := NewSkillEngine()

	builtinSkills, err := LoadBuiltinSkills(m.cfg.SkillsDir)
	if err != nil {
		log.Printf("[管理器] 机器人 %s 加载内置 Skill 失败: %v", b.ID, err)
	}
	for _, s := range builtinSkills {
		var existing db.Skill
		result := db.DB.Where("bot_id = ? AND name = ?", b.ID, s.Name).First(&existing)
		if result.Error != nil {
			db.DB.Create(&db.Skill{
				BotID:        b.ID,
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

	skillMap := make(map[string]Skill)

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
			Platform:         b.PlatformType,
			LLMProviderID:    b.LLMProviderID,
			LLMModel:         b.LLMModel,
			LLMTemperature:   temp,
			LLMMaxTokens:     maxTokens,
			MaxSessionTokens: lm,
			Enabled:          b.Enabled,
			GoJudgeEndpoint:  m.cfg.GoJudgeEndpoint,
		},
		Platform:        platformClient,
		SkillEng:        skillEng,
		SessionMgr:      NewSessionManager(b.ID),
		WeaviateClient:  m.weaviateClient,
		BuiltinEmbedder: m.builtinEmbedder,
		TaskEngine:      m.taskEngine,
	}

	platformClient.SetStatusCallback(func(status string) {
		db.DB.Model(&db.Bot{}).Where("id = ?", b.ID).Update("status", status)
	})

	log.Printf("[管理器] 启动机器人 %s (platform=%s)", b.ID, b.PlatformType)

	if err := platformClient.Start(); err != nil {
		log.Printf("[管理器] 机器人 %s 连接失败: %v", b.ID, err)
		return fmt.Errorf("机器人 %s 连接失败: %w", b.ID, err)
	}

	// Register message handler after successful connection
	instance.registerMessageHandler()

	m.AddBot(b.ID, instance)
	log.Printf("[管理器] 机器人 %s 启动成功", b.ID)
	return nil
}

func (b *BotInstance) registerMessageHandler() {
	b.Platform.SetMessageHandler(func(msg *Message) {
		if err := b.HandleMessage(msg); err != nil {
			log.Printf("机器人 %s 消息处理失败: %v", b.ID, err)
		}
	})
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
	instance.Platform.Stop()
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

	// Add scheduled_task tool if task engine is available
	if b.TaskEngine != nil {
		tools = append(tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "scheduled_task",
				Description: "管理当前会话的定时任务：创建定时任务、取消任务、列出任务。注意：必须调用此工具才能真正创建/取消任务，不要编造任务 ID 或假装任务已创建。创建任务时根据对话语境判断 route 参数：如果需要 AI 加工结果则用 \"llm\"，如果只是直接推送消息则用 \"direct\"。run_at 参数必须包含时区（如 \"2026-07-11T01:53:00+08:00\"）。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"create", "cancel", "list"},
							"description": "操作类型",
						},
						"task_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"interval", "cron", "once"},
							"description": "定时类型：interval=间隔执行, cron=表达式, once=一次性",
						},
						"interval_sec": map[string]interface{}{
							"type":        "number",
							"description": "间隔秒数（task_type=interval 时必填）",
						},
						"cron_expr": map[string]interface{}{
							"type":        "string",
							"description": "cron 表达式（task_type=cron 时必填）",
						},
						"run_at": map[string]interface{}{
							"type":        "string",
							"description": "执行时间 ISO8601（task_type=once 时必填）",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "任务内容。如果 route=direct 则为直接发送给用户的消息文本；如果 route=llm 则为给 LLM 的指令",
						},
						"route": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"llm", "direct"},
							"description": "执行路由：llm=走 LLM 处理后再发结果, direct=直接发消息",
						},
						"task_id": map[string]interface{}{
							"type":        "string",
							"description": "任务 ID（cancel 时必填）",
						},
					},
					"required": []string{"operation"},
				},
			},
		})
	}

	// Add search_knowledge tool if Weaviate client is available
	if b.WeaviateClient != nil {
		var kbDesc string
		var bindings []db.BotKnowledgeBinding
		db.DB.Where("bot_id = ?", b.ID).Find(&bindings)
		if len(bindings) > 0 {
			var kbParts []string
			for _, binding := range bindings {
				var kb db.KnowledgeBase
				if err := db.DB.Where("id = ?", binding.KnowledgeBaseID).First(&kb).Error; err == nil {
					entry := kb.Name + " (" + kb.ID + ")"
					if kb.Description != "" {
						entry += "：" + kb.Description
					}
					if kb.AutoSummary != "" {
						entry += "（内容概要：" + kb.AutoSummary + "）"
					}
					kbParts = append(kbParts, entry)
				}
			}
			if len(kbParts) > 0 {
				kbDesc = "当前可检索知识库合集：\n" + strings.Join(kbParts, "\n")
			}
		}
		if kbDesc == "" {
			kbDesc = "从知识库中检索相关内容。"
		}

		tools = append(tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "search_knowledge",
				Description: kbDesc,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "搜索查询",
						},
						"kb_ids": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "限定知识库 ID 列表（可选，默认检索所有绑定的知识库）",
						},
						"top_k": map[string]interface{}{
							"type":        "number",
							"description": "返回结果数量（默认 5）",
						},
					},
					"required": []string{"query"},
				},
			},
		})
	}

	return tools
}

func loadGoJudgeTools(botID string, skill *Skill, tools *[]llm.ToolDefinition) {
	var dbSkill db.Skill
	if err := db.DB.Where("bot_id = ? AND name = ?", botID, skill.Name).First(&dbSkill).Error; err != nil {
		log.Printf("[go-judge] 未找到对应 Skill: %s", skill.Name)
		return
	}
	goJudgeTools, err := skilltool.ListEnabledToolsBySkill(botID, dbSkill.ID)
	if err != nil {
		log.Printf("[go-judge] 加载 Tool 失败: %v", err)
		return
	}
	log.Printf("[go-judge] 加载 %d 个 Tool (skill=%s)", len(goJudgeTools), skill.Name)
	for _, t := range goJudgeTools {
		fnName := "gjtool__" + fmt.Sprintf("%d", dbSkill.ID) + "_" + t.Name
		log.Printf("[go-judge] 添加 Tool: name=%s fn=%s", t.Name, fnName)
		desc := t.Name
		if t.Prompt != "" {
			desc += ": " + t.Prompt
		}
		desc += fmt.Sprintf(" (语言: %s)", t.Language)
		params := buildToolParams(t.InputParams)
		*tools = append(*tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        fnName,
				Description: desc,
				Parameters:  params,
			},
		})
	}
}

func buildToolParams(inputParams string) map[string]interface{} {
	properties := make(map[string]interface{})
	for i := 1; i <= 32; i++ {
		key := fmt.Sprintf("value%d", i)
		properties[key] = map[string]interface{}{
			"type":        "string",
			"description": fmt.Sprintf("第 %d 个参数", i),
		}
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
}

func (b *BotInstance) executeToolCall(tc llm.ToolCall, shellExec *agent.ShellExecutor, sessionKey string, streamFn func(string)) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "参数解析失败: " + err.Error()
	}

	switch {
	case tc.Function.Name == "scheduled_task":
		if b.TaskEngine == nil {
			return "定时任务引擎未初始化"
		}
		handler := task.NewScheduledTaskHandler(b.TaskEngine.Store(), b.TaskEngine)
		return handler.Handle(json.RawMessage(tc.Function.Arguments), b.ID, sessionKey)

	case tc.Function.Name == "search_knowledge":
		query, _ := args["query"].(string)
		if query == "" {
			return "缺少 query 参数"
		}

		kbIDsRaw, _ := args["kb_ids"].([]interface{})
		var kbIDs []string
		for _, id := range kbIDsRaw {
			if s, ok := id.(string); ok {
				kbIDs = append(kbIDs, s)
			}
		}

		topK := 5
		if topKFloat, ok := args["top_k"].(float64); ok {
			topK = int(topKFloat)
			if topK > 20 {
				topK = 20
			}
		}

		// If no kb_ids specified, use bot's bound knowledge bases
		if len(kbIDs) == 0 {
			var bindings []db.BotKnowledgeBinding
			db.DB.Where("bot_id = ?", b.ID).Find(&bindings)
			for _, binding := range bindings {
				kbIDs = append(kbIDs, binding.KnowledgeBaseID)
			}
		}

		if len(kbIDs) == 0 {
			return "未绑定任何知识库，无法检索"
		}

		// Use first KB's embedding config
		var firstKB db.KnowledgeBase
		var embedErr error
		if err := db.DB.Where("id = ?", kbIDs[0]).First(&firstKB).Error; err != nil {
			return fmt.Sprintf("知识库 %s 未找到: %v", kbIDs[0], err)
		}

		var vectors [][]float32
		if firstKB.EmbeddingMode == "builtin" && b.BuiltinEmbedder != nil {
			vectors, embedErr = b.BuiltinEmbedder.Embed("", "", []string{query})
		} else if firstKB.EmbeddingMode == "provider" {
			if firstKB.EmbeddingProviderID == "" || firstKB.EmbeddingModel == "" {
				return "知识库未配置 embedding 提供商或模型，无法检索"
			}
			embedder := knowledge.NewEmbedder()
			vectors, embedErr = embedder.Embed(firstKB.EmbeddingProviderID, firstKB.EmbeddingModel, []string{query})
		} else {
			return "知识库未配置有效的 embedding 模式，无法检索"
		}
		if embedErr != nil {
			return "向量化查询失败: " + embedErr.Error()
		}
		queryVector := vectors[0]

		results, err := b.WeaviateClient.HybridSearch(context.Background(), query, queryVector, kbIDs, topK, 0.5)
		if err != nil {
			return "检索失败: " + err.Error()
		}

		if len(results) == 0 {
			return "未检索到相关内容"
		}

		var buf strings.Builder
		buf.WriteString(fmt.Sprintf("找到 %d 条相关结果：\n\n", len(results)))
		for i, r := range results {
			buf.WriteString(fmt.Sprintf("--- 结果 %d (相关度: %.2f) ---\n", i+1, r.Score))
			if r.DocTitle != "" {
				buf.WriteString(fmt.Sprintf("标题: %s\n", r.DocTitle))
			}
			buf.WriteString(fmt.Sprintf("来源: %s\n", r.SourceFile))
			buf.WriteString(fmt.Sprintf("内容: %s\n\n", r.Content))
		}
		return buf.String()

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
		parts := strings.SplitN(strings.TrimPrefix(tc.Function.Name, "gjtool__"), "_", 2)
		if len(parts) != 2 {
			return "无效的 Tool 名称: " + tc.Function.Name
		}
		var skillID uint
		if _, err := fmt.Sscanf(parts[0], "%d", &skillID); err != nil {
			return "无效的 Skill ID: " + parts[0]
		}
		toolName := parts[1]

		var gjt db.GoJudgeTool
		if err := db.DB.Where("skill_id = ? AND name = ?", skillID, toolName).First(&gjt).Error; err != nil {
			return "go-judge Tool 未找到"
		}

		streamFn(fmt.Sprintf("\n\n🛠️ 执行 Tool: %s (%s)\n", gjt.Name, gjt.Language))

		var cliArgs []string
		for i := 1; i <= 32; i++ {
			key := fmt.Sprintf("value%d", i)
			if v, ok := args[key].(string); ok {
				cliArgs = append(cliArgs, v)
			}
		}

		stdinContent := ""
		if len(args) > 0 {
			argsJSON, err := json.Marshal(args)
			if err == nil {
				stdinContent = string(argsJSON)
			}
		}

		executor := skilltool.NewExecutor(b.Config.GoJudgeEndpoint)
		resp, err := executor.Execute(&skilltool.ExecuteRequest{
			Lang:  gjt.Language,
			Src:   gjt.Code,
			Stdin: stdinContent,
			Args:  cliArgs,
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

		var result string
		var err error
		if mcp.Type == "command" {
			var cmdArgs []string
			if mcp.Args != "" {
				json.Unmarshal([]byte(mcp.Args), &cmdArgs)
			}
			var envMap map[string]string
			if mcp.Env != "" {
				json.Unmarshal([]byte(mcp.Env), &envMap)
			}
			client := agent.NewLocalMCPClient(mcp.Name, mcp.Command, cmdArgs, envMap)
			result, err = client.Call(toolName, args)
		} else {
			client := agent.NewMCPClient(mcp.Name, mcp.Endpoint)
			result, err = client.Call(toolName, args)
		}
		if err != nil {
			return "MCP 调用失败: " + err.Error()
		}
		return result
	}
}
