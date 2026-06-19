// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OpenAIClient struct {
	endpoint    string
	apiKey      string
	model       string
	Temperature float64
	MaxTokens   int
	client      *http.Client
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []ChatMessage   `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content      string     `json:"content"`
			ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func NewOpenAIClient(endpoint, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		endpoint: strings.TrimRight(endpoint, "/") + "/chat/completions",
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *OpenAIClient) Chat(messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	req := openAIRequest{
		Model:    c.model,
		Messages: messages,
	}

	if len(tools) > 0 {
		req.Tools = tools
	}
	if c.Temperature != 0 {
		req.Temperature = c.Temperature
	}
	if c.MaxTokens != 0 {
		req.MaxTokens = c.MaxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error [%d]: %s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := openAIResp.Choices[0]
	log.Printf("[LLM 原始响应] model=%s content=%s tool_calls=%d", c.model, choice.Message.Content, len(choice.Message.ToolCalls))
	return &ChatResponse{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
	}, nil
}

func (c *OpenAIClient) ChatStream(messages []ChatMessage, tools []ToolDefinition) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		defer close(ch)
		resp, err := c.Chat(messages, tools)
		if err != nil {
			return
		}
		ch <- resp.Content
	}()
	return ch, nil
}

func EstimateTokens(text string) int {
	return len(text) / 4
}
