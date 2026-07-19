// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

type PlatformClient interface {
	Start() error
	Stop()
	Status() string
	SetStatusCallback(cb func(status string))
	SetMessageHandler(handler func(msg *Message))
	Connected() bool
	SendReply(reqID string, content string) error
	SendStreamChunk(reqID, streamID, content string, finish bool) error
	SendActiveMsg(reqID, chatID string, chatType int, content string) error
	Translator() Translator
}
