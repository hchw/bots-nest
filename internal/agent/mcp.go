// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
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

// LocalMCPClient communicates with a local MCP Server via its subprocess stdio.
type LocalMCPClient struct {
	name    string
	command string
	args    []string
	reqID   int
}

func NewLocalMCPClient(name, command string, args []string) *LocalMCPClient {
	return &LocalMCPClient{
		name:    name,
		command: command,
		args:    args,
	}
}

func (m *LocalMCPClient) nextID() *int {
	m.reqID++
	return &m.reqID
}

func (m *LocalMCPClient) Name() string {
	return m.name
}

// execWithTimeout starts the subprocess, sends the request via stdin, and reads the response from stdout.
func (m *LocalMCPClient) execWithTimeout(ctx context.Context, req *jrpcRequest) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, m.command, m.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdin pipe 失败: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout pipe 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动子进程失败: %w", err)
	}

	stdin.Write(body)
	stdin.Close()

	out, _ := io.ReadAll(stdout)
	errOut, _ := io.ReadAll(stderr)

	waitErr := cmd.Wait()
	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("MCP 请求超时（%s）", req.Method)
		}
		errMsg := ""
		if len(errOut) > 0 {
			errMsg = ": " + string(errOut)
		}
		return nil, fmt.Errorf("子进程异常退出: %v%s", waitErr, errMsg)
	}

	if req.ID == nil {
		return nil, nil
	}

	var jr jrpcResponse
	if err := json.Unmarshal(out, &jr); err != nil {
		return nil, fmt.Errorf("MCP 响应 JSON 解析失败: %w", err)
	}
	if jr.Error != nil {
		return nil, fmt.Errorf("MCP %s 返回错误 (%d): %s", req.Method, jr.Error.Code, jr.Error.Message)
	}

	return jr.Result, nil
}

func (m *LocalMCPClient) initSession(ctx context.Context) error {
	_, err := m.execWithTimeout(ctx, &jrpcRequest{
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
	})
	if err != nil {
		return err
	}

	_, err = m.execWithTimeout(ctx, &jrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})
	return err
}

func (m *LocalMCPClient) DiscoverTools() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.initSession(ctx); err != nil {
		return "[]", err
	}

	raw, err := m.execWithTimeout(ctx, &jrpcRequest{
		JSONRPC: "2.0",
		ID:      m.nextID(),
		Method:  "tools/list",
	})
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

func (m *LocalMCPClient) Call(toolName string, args map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.initSession(ctx); err != nil {
		return "", err
	}

	raw, err := m.execWithTimeout(ctx, &jrpcRequest{
		JSONRPC: "2.0",
		ID:      m.nextID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	})
	if err != nil {
		return "", err
	}

	return string(raw), nil
}
