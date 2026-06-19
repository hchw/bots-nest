// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"github.com/hchw/bots-nest/internal/llm"
)

type MCPClient struct {
	name     string
	endpoint string
	client   *llm.OpenAIClient
}

func NewMCPClient(name, endpoint string) *MCPClient {
	return &MCPClient{
		name:     name,
		endpoint: endpoint,
	}
}

func (m *MCPClient) Name() string {
	return m.name
}

func (m *MCPClient) DiscoverTools() (string, error) {
	resp, err := http.Get(m.endpoint + "/tools")
	if err != nil {
		resp, err = http.Get(m.endpoint)
		if err != nil {
			return "[]", fmt.Errorf("无法连接 MCP 端点: %w", err)
		}
	}
	defer resp.Body.Close()

	var tools []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return "[]", nil
	}
	data, _ := json.Marshal(tools)
	return string(data), nil
}

func (m *MCPClient) Call(toolName string, args map[string]interface{}) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", fmt.Errorf("MCP 请求序列化失败: %w", err)
	}

	resp, err := http.Post(m.endpoint+"/call", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("MCP 调用失败: %w", err)
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("MCP 读取响应失败: %w", err)
	}

	return string(result), nil
}
