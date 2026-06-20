// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MCPClient struct {
	name     string
	endpoint string
	reqID    int
}

func NewMCPClient(name, endpoint string) *MCPClient {
	return &MCPClient{
		name:     name,
		endpoint: endpoint,
	}
}

func (m *MCPClient) nextID() *int {
	m.reqID++
	return &m.reqID
}

func (m *MCPClient) Name() string {
	return m.name
}

// jrpcRequest is a generic JSON-RPC 2.0 request body.
type jrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int        `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jrpcResponse is a generic JSON-RPC 2.0 response (partial).
type jrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jrpcError      `json:"error,omitempty"`
}

type jrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// doJSONRPC sends a JSON-RPC POST with the streamable_http headers.
// On the first call (initialize), it extracts the Mcp-Session-Id from the
// response headers and returns it.  On subsequent calls the caller passes the
// session id back in via the session parameter.
func (m *MCPClient) doJSONRPC(req *jrpcRequest, session string) (string, json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, err
	}

	httpReq, err := http.NewRequest("POST", m.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		httpReq.Header.Set("Mcp-Session-Id", session)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("MCP %s 请求失败: %w", req.Method, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("MCP 读取响应失败: %w", err)
	}

	if session == "" && req.Method == "initialize" {
		session = resp.Header.Get("Mcp-Session-Id")
	}

	// Notifications have no response body
	if req.ID == nil {
		return session, nil, nil
	}

	// Some MCP servers respond with SSE format (event:/data:) for initialize.
	// Parse out the JSON from the data field if needed.
	payload := raw
	if isSSE(raw) {
		payload = parseSSEData(raw)
	}

	var jr jrpcResponse
	if err := json.Unmarshal(payload, &jr); err != nil {
		return session, nil, fmt.Errorf("MCP 响应 JSON 解析失败: %w", err)
	}
	if jr.Error != nil {
		return session, nil, fmt.Errorf("MCP %s 返回错误 (%d): %s", req.Method, jr.Error.Code, jr.Error.Message)
	}

	return session, jr.Result, nil
}

// isSSE checks whether the response body is in SSE (Server-Sent Events) format.
func isSSE(raw []byte) bool {
	return len(raw) > 0 && (raw[0] == 'e' || raw[0] == 'd' || raw[0] == 'i')
}

// parseSSEData extracts the JSON payload from SSE data: lines.
func parseSSEData(raw []byte) []byte {
	var data strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(line[5:]))
		}
	}
	if data.Len() > 0 {
		return []byte(data.String())
	}
	return raw
}

// initSession performs the MCP initialization handshake and returns a session id.
func (m *MCPClient) initSession() (string, error) {
	session, _, err := m.doJSONRPC(&jrpcRequest{
		JSONRPC: "2.0",
		ID:      m.nextID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "qw-bot",
				"version": "1.0",
			},
		},
	}, "")
	if err != nil {
		return "", err
	}

	_, _, err = m.doJSONRPC(&jrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}, session)
	return session, err
}

func (m *MCPClient) DiscoverTools() (string, error) {
	session, err := m.initSession()
	if err != nil {
		return "[]", err
	}

	_, raw, err := m.doJSONRPC(&jrpcRequest{
		JSONRPC: "2.0",
		ID:      m.nextID(),
		Method:  "tools/list",
	}, session)
	if err != nil {
		return "[]", err
	}

	var listResult struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(raw, &listResult); err != nil {
		return "[]", nil
	}

	mapped := make([]map[string]interface{}, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		var rawTool struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			InputSchema interface{} `json:"inputSchema"`
		}
		if err := json.Unmarshal(t, &rawTool); err != nil {
			continue
		}
		mapped = append(mapped, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        rawTool.Name,
				"description": rawTool.Description,
				"parameters":  rawTool.InputSchema,
			},
		})
	}

	data, _ := json.Marshal(mapped)
	return string(data), nil
}

func (m *MCPClient) Call(toolName string, args map[string]interface{}) (string, error) {
	session, err := m.initSession()
	if err != nil {
		return "", err
	}

	_, raw, err := m.doJSONRPC(&jrpcRequest{
		JSONRPC: "2.0",
		ID:      m.nextID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}, session)
	if err != nil {
		return "", err
	}

	return string(raw), nil
}
