package knowledge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hchw/bots-nest/internal/db"
)

type Embedder interface {
	Embed(providerID, model string, texts []string) ([][]float32, error)
}

type ProviderEmbedder struct{}

func NewEmbedder() Embedder {
	return &ProviderEmbedder{}
}

type openAIEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (e *ProviderEmbedder) Embed(providerID, model string, texts []string) ([][]float32, error) {
	var provider db.LLMProvider
	if err := db.DB.Where("id = ?", providerID).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("embedding provider %s 未找到: %w", providerID, err)
	}

	endpoint := strings.TrimRight(provider.Endpoint, "/") + "/embeddings"

	body := openAIEmbedRequest{
		Input: texts,
		Model: model,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 embedding API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API 返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp openAIEmbedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(embedResp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding 结果数量不匹配: 期望 %d, 得到 %d", len(texts), len(embedResp.Data))
	}

	result := make([][]float32, len(embedResp.Data))
	for i, d := range embedResp.Data {
		vec := make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			vec[j] = float32(v)
		}
		result[i] = vec
	}
	return result, nil
}
