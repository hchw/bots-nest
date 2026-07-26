// SPDX-License-Identifier: Apache-2.0
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
