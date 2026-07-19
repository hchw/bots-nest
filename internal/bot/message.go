// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

type Message struct {
	SenderID         string
	Content          string
	ConversationType string // "single" or "group"
	GroupID          string
	ReplyToken       string // reqID for WeCom, sessionWebhook for DingTalk
	MsgType          string // "text" (future: "image", "file", etc.)
}
