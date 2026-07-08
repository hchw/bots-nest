// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package llm

import (
	"bufio"
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
	Stream      bool            `json:"stream,omitempty"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role,omitempty"`
			Content   string `json:"content,omitempty"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function,omitempty"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content      string     `json:"content"`
			ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Type    string `json:"type,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
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

	if openAIResp.Error != nil && openAIResp.Error.Message != "" {
		return nil, fmt.Errorf("API error [%s]: %s", openAIResp.Error.Type, openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response (status=%d): %s", resp.StatusCode, string(respBody))
	}

	choice := openAIResp.Choices[0]
	log.Printf("[LLM 原始响应] model=%s content=%s tool_calls=%d", c.model, choice.Message.Content, len(choice.Message.ToolCalls))
	return &ChatResponse{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
	}, nil
}

func (c *OpenAIClient) ChatStream(messages []ChatMessage, tools []ToolDefinition) (<-chan StreamEvent, error) {
	req := openAIRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	if len(tools) > 0 {
		req.Tools = tools
		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, t.Function.Name)
		}
		log.Printf("[ChatStream] 发送 tools=%v", toolNames)
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

	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error [%d]: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamEvent)
	go c.readSSEStream(resp, ch)
	return ch, nil
}

func (c *OpenAIClient) readSSEStream(resp *http.Response, ch chan<- StreamEvent) {
	defer close(ch)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*128), 1024*128)

	type accToolCall struct {
		index     int
		id        string
		type_     string
		name      string
		arguments string
	}
	toolCallAcc := make(map[int]*accToolCall)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[ChatStream] 解析流式块失败: %v, data=%s", err, data)
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.Delta.Content != "" {
			ch <- StreamEvent{Content: choice.Delta.Content}
		}

		for _, tc := range choice.Delta.ToolCalls {
			acc, ok := toolCallAcc[tc.Index]
			if !ok {
				acc = &accToolCall{index: tc.Index}
				toolCallAcc[tc.Index] = acc
			}
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Type != "" {
				acc.type_ = tc.Type
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				acc.arguments += tc.Function.Arguments
			}
		}

		if choice.FinishReason != nil {
			log.Printf("[ChatStream] finish_reason=%s", *choice.FinishReason)
			switch *choice.FinishReason {
			case "stop", "length":
				ch <- StreamEvent{Done: true}
			case "tool_calls":
				var toolCalls []ToolCall
				for i := 0; i < len(toolCallAcc); i++ {
					acc := toolCallAcc[i]
					if acc != nil {
						toolCalls = append(toolCalls, ToolCall{
							ID:   acc.id,
							Type: acc.type_,
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name:      acc.name,
								Arguments: acc.arguments,
							},
						})
					}
				}
				ch <- StreamEvent{ToolCalls: toolCalls, Done: true}
			default:
				log.Printf("[ChatStream] 未处理的 finish_reason=%s", *choice.FinishReason)
				ch <- StreamEvent{Done: true}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[ChatStream] 读取流式响应错误: %v", err)
	}
}

func EstimateTokens(text string) int {
	return len(text) / 4
}
