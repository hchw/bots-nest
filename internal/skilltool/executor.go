package skilltool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ExecuteRequest struct {
	Lang string `json:"lang"`
	Src  string `json:"src"`
	Stdin string `json:"stdin,omitempty"`
}

type ExecuteResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Executor struct {
	endpoint string
	client   *http.Client
}

func NewExecutor(endpoint string) *Executor {
	return &Executor{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *Executor) Execute(req *ExecuteRequest) (*ExecuteResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", e.endpoint+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go-judge error [%d]: %s", resp.StatusCode, string(respBody))
	}

	var result ExecuteResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

func (e *Executor) ExecuteWithStdin(lang, src, stdin string) (*ExecuteResponse, error) {
	return e.Execute(&ExecuteRequest{
		Lang:  lang,
		Src:   src,
		Stdin: stdin,
	})
}

func SupportedLanguages() []string {
	return []string{
		"c", "cpp", "go", "java", "python3", "javascript",
	}
}
