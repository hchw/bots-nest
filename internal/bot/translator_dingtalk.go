// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"encoding/json"
	"log"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

type DingTalkTranslator struct{}

func NewDingTalkTranslator() *DingTalkTranslator {
	return &DingTalkTranslator{}
}

func (t *DingTalkTranslator) Platform() string {
	return "dingtalk"
}

func (t *DingTalkTranslator) ParseIncoming(raw []byte) (*Message, error) {
	var data chatbot.BotCallbackDataModel
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	convType := "single"
	if data.ConversationType == "2" {
		convType = "group"
	}

	content := data.Text.Content

	if content == "" {
		log.Printf("钉钉收到空消息, 跳过处理")
		return nil, nil
	}

	if data.Msgtype != "text" {
		log.Printf("钉钉收到不支持的消息类型: %s", data.Msgtype)
		return nil, nil
	}

	return &Message{
		SenderID:         data.SenderId,
		Content:          content,
		ConversationType: convType,
		GroupID:          data.ConversationId,
		ReplyToken:       data.SessionWebhook,
		MsgType:          data.Msgtype,
	}, nil
}
