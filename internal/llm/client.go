// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package llm

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolDefinition struct {
	Type       string     `json:"type"`
	Function   FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type StreamEvent struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Done      bool       `json:"done"`
}

type Client interface {
	Chat(messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error)
	ChatStream(messages []ChatMessage, tools []ToolDefinition) (<-chan StreamEvent, error)
}
