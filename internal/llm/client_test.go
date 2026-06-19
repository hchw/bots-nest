// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package llm

import (
	"testing"
)

type mockClient struct {
	response *ChatResponse
	err      error
}

func (m *mockClient) Chat(messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	return m.response, m.err
}

func (m *mockClient) ChatStream(messages []ChatMessage, tools []ToolDefinition) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		if m.response != nil {
			ch <- m.response.Content
		}
		close(ch)
	}()
	return ch, m.err
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"short", "hello", 1},
		{"medium", "hello world this is a test", 6},
		{"chinese", "你好世界", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, 期望 %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewOpenAIClient(t *testing.T) {
	client := NewOpenAIClient("https://api.openai.com/v1", "sk-test", "gpt-4o")
	if client == nil {
		t.Fatal("client 不应为 nil")
	}
}

func TestMockClient(t *testing.T) {
	client := &mockClient{
		response: &ChatResponse{Content: "Hello"},
	}
	resp, err := client.Chat(nil, nil)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "Hello" {
		t.Errorf("期望 Content=Hello，得到 %s", resp.Content)
	}
}
