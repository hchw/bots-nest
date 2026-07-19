// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hchw/bots-nest/internal/agent"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/llm"
)

func (b *BotInstance) HandleMessage(msg *Message) error {
	platform := b.Platform.Translator().Platform()
	sessionKey := fmt.Sprintf("%s:%s:%s:%s", b.ID, platform, msg.SenderID, msg.ConversationType)

	groupID := msg.GroupID

	session, err := b.SessionMgr.GetOrCreateWithKey(sessionKey, msg.SenderID, msg.SenderID, msg.ConversationType, groupID)
	if err != nil {
		return fmt.Errorf("获取/创建会话失败: %w", err)
	}

	userContent := ""
	switch msg.MsgType {
	case "text":
		userContent = msg.Content
	default:
		log.Printf("机器人 %s 收到不支持的消息类型: %s", b.ID, msg.MsgType)
		return nil
	}

	if userContent == "" {
		log.Printf("机器人 %s 收到空消息, 跳过处理", b.ID)
		return nil
	}

	if err := b.SessionMgr.AddMessage(session.SessionKey, "user", userContent, llm.EstimateTokens(userContent)); err != nil {
		return fmt.Errorf("存储用户消息失败: %w", err)
	}

	switch cmd := strings.TrimSpace(userContent); cmd {
	case "/clear":
		if err := b.SessionMgr.ClearSession(session.SessionKey); err != nil {
			return fmt.Errorf("清空会话失败: %w", err)
		}
		b.Platform.SendReply(msg.ReplyToken, "会话已清空")
		return nil
	case "/compress":
		return b.handleCompressCommand(msg.ReplyToken, session.SessionKey)
	}

	var msgs []llm.ChatMessage

	msgs = append(msgs, llm.ChatMessage{Role: "system", Content: DefaultSystemPrompt})

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

	tools := b.buildTools()

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

	shellExec := agent.NewShellExecutor(nil, 30*time.Second, 4096)
	reqID := msg.ReplyToken
	streamID := fmt.Sprintf("s_%d", time.Now().UnixNano())

	var finalReply string
	maxIterations := 10

	var displayBuf strings.Builder
	var lastFlushedLen int
	var lastFlush time.Time

	flushDisplay := func(finish bool) {
		content := displayBuf.String()
		if len(content) > 0 || finish {
			b.Platform.SendStreamChunk(reqID, streamID, content, finish)
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
	log.Printf("[loop] 开始循环, maxIterations=%d, msgs=%d, tools=%d", maxIterations, len(msgs), len(tools))

	for iter := 0; iter < maxIterations; iter++ {
		log.Printf("[loop] iter=%d/10 开始 ChatStream msgs=%d tools=%d", iter, len(msgs), len(tools))
		eventCh, err := llmClient.ChatStream(msgs, tools)
		if err != nil {
			log.Printf("[loop] iter=%d ChatStream 出错: %v", iter, err)
			writeDisplay("\n处理出错: " + err.Error())
			flushDisplay(true)
			return fmt.Errorf("LLM 流式调用失败: %w", err)
		}

		var contentBuf strings.Builder
		var toolCalls []llm.ToolCall

		log.Printf("[loop] iter=%d 开始读取流", iter)
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
			var names []string
			for _, tc := range toolCalls {
				names = append(names, tc.Function.Name)
			}
			log.Printf("[loop] iter=%d 收到 tool 调用: %v, content=%q", iter, names, contentBuf.String())
			flushDisplay(false)
		} else {
			log.Printf("[loop] iter=%d 无 tool 调用, content=%q, 结束循环", iter, contentBuf.String())
		}

		if len(toolCalls) == 0 {
			if contentBuf.Len() > 0 {
				log.Printf("[loop] iter=%d 无 tool 调用, 直接使用流式内容", iter)
				finalReply = contentBuf.String()
			} else {
				log.Printf("[loop] iter=%d 无 tool 调用且流式内容为空, 纯文本模式重新请求", iter)
				resp, err := llmClient.Chat(msgs, nil)
				if err != nil {
					log.Printf("[loop] iter=%d 纯文本 Chat 出错: %v", iter, err)
					finalReply = ""
				} else {
					finalReply = resp.Content
					writeDisplay(resp.Content)
				}
			}
			break
		}

		msgs = append(msgs, llm.ChatMessage{
			Role:      "assistant",
			Content:   contentBuf.String(),
			ToolCalls: toolCalls,
		})

		for _, tc := range toolCalls {
			var result string
			if tc.Function.Name == "activate_skill" {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					result = "参数解析失败: " + err.Error()
					log.Printf("[loop] iter=%d activate_skill 参数解析失败: %v", iter, err)
				} else {
					skillName, _ := args["skill_name"].(string)
					log.Printf("[loop] iter=%d activate_skill skill=%s", iter, skillName)
					if skill := b.SkillEng.Lookup(b.ID, skillName); skill != nil {
						result = skill.SystemPrompt
						if skill.Tools != "" {
							var skillTools []llm.ToolDefinition
							if err := json.Unmarshal([]byte(skill.Tools), &skillTools); err == nil {
								tools = append(tools, skillTools...)
							}
						}
						loadGoJudgeTools(b.ID, skill, &tools)
						log.Printf("[loop] iter=%d activate_skill 完成, tools现在有 %d 个", iter, len(tools))
					} else {
						result = "未找到技能: " + skillName
						log.Printf("[loop] iter=%d 未找到技能: %s", iter, skillName)
					}
				}
			} else {
				log.Printf("[loop] iter=%d 执行 tool: %s args=%s", iter, tc.Function.Name, tc.Function.Arguments)
				result = b.executeToolCall(tc, shellExec, session.SessionKey, func(chunk string) {
					writeDisplay(chunk)
				})
				log.Printf("[loop] iter=%d tool %s 结果: %s", iter, tc.Function.Name, result)
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
		log.Printf("[loop] 超时, 结束")
	}
	log.Printf("[loop] 最终回复: %s", finalReply)

	displayBuf.Reset()
	displayBuf.WriteString(finalReply)
	flushDisplay(true)

	if err := b.SessionMgr.AddMessage(session.SessionKey, "assistant", finalReply, llm.EstimateTokens(finalReply)); err != nil {
		return fmt.Errorf("存储回复失败: %w", err)
	}

	b.autoCompressIfNeeded(session.SessionKey, reqID)

	return nil
}

func (b *BotInstance) handleCompressCommand(replyToken string, sessionKey string) error {
	history, err := b.SessionMgr.GetHistory(sessionKey, 100)
	if err != nil {
		return fmt.Errorf("获取历史消息失败: %w", err)
	}
	if len(history) == 0 {
		b.Platform.SendReply(replyToken, "没有可压缩的消息")
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

	b.Platform.SendReply(replyToken, "已压缩会话")
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
	if err := b.Platform.SendReply(reqID, notifyContent); err != nil {
		log.Printf("自动压缩通知发送失败: %v", err)
	}
}


