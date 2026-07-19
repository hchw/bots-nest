// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"encoding/json"
	"log"
)

type WeComTranslator struct{}

func NewWeComTranslator() *WeComTranslator {
	return &WeComTranslator{}
}

func (t *WeComTranslator) Platform() string {
	return "wecom"
}

func (t *WeComTranslator) ParseIncoming(raw []byte) (*Message, error) {
	var msg WeComMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}

	if msg.Cmd != cmdMsgCallback {
		return nil, nil
	}

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

	content := ""
	switch body.MsgType {
	case "text":
		if body.Text != nil {
			content = body.Text.Content
		}
	default:
		log.Printf("WeCom 收到不支持的消息类型: %s", body.MsgType)
		return nil, nil
	}

	if content == "" {
		log.Printf("WeCom 收到空消息, 跳过处理")
		return nil, nil
	}

	return &Message{
		SenderID:         userID,
		Content:          content,
		ConversationType: convType,
		GroupID:          groupID,
		ReplyToken:       msg.Headers.ReqID,
		MsgType:          body.MsgType,
	}, nil
}
