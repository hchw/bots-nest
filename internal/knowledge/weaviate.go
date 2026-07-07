package knowledge

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const KnowledgeChunkClass = "KnowledgeChunk"

type Chunk struct {
	Content    string    `json:"content"`
	KBID       string    `json:"kb_id"`
	SourceFile string    `json:"source_file"`
	ChunkIndex int       `json:"chunk_index"`
	DocTitle   string    `json:"doc_title"`
	TokenCount int       `json:"token_count"`
	Vector     []float32 `json:"-"`
}

type SearchResult struct {
	Content    string  `json:"content"`
	KBID       string  `json:"kb_id"`
	SourceFile string  `json:"source_file"`
	ChunkIndex int     `json:"chunk_index"`
	DocTitle   string  `json:"doc_title"`
	Score      float64 `json:"score"`
}

type WeaviateClient struct {
	client   *weaviate.Client
	endpoint string
}

func NewWeaviateClient(endpoint, scheme, apiKey string) (*WeaviateClient, error) {
	cfg := weaviate.Config{
		Host:   endpoint,
		Scheme: scheme,
	}
	if apiKey != "" {
		cfg.AuthConfig = auth.ApiKey{Value: apiKey}
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 Weaviate 客户端失败: %w", err)
	}

	return &WeaviateClient{client: client, endpoint: endpoint}, nil
}

func (c *WeaviateClient) WaitForReady(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/v1/.well-known/ready", c.endpoint)
	for i := 0; i < 30; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if err == nil {
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("Weaviate 未在 30s 内就绪")
}

func (c *WeaviateClient) CreateCollection(ctx context.Context) error {
	exists, err := c.client.Schema().ClassExistenceChecker().WithClassName(KnowledgeChunkClass).Do(ctx)
	if err != nil {
		return fmt.Errorf("检查集合是否存在失败: %w", err)
	}
	if exists {
		log.Println("[Weaviate] KnowledgeChunk 集合已存在，跳过创建")
		return nil
	}

	class := &models.Class{
		Class:      KnowledgeChunkClass,
		Vectorizer: "none",
		Properties: []*models.Property{
			{Name: "content", DataType: []string{"text"}, Description: "chunk 文本内容"},
			{Name: "kb_id", DataType: []string{"string"}, Description: "知识库 ID"},
			{Name: "source_file", DataType: []string{"string"}, Description: "来源文件名"},
			{Name: "chunk_index", DataType: []string{"int"}, Description: "分片序号"},
			{Name: "doc_title", DataType: []string{"string"}, Description: "文档标题"},
			{Name: "token_count", DataType: []string{"int"}, Description: "预估 token 数"},
		},
	}

	err = c.client.Schema().ClassCreator().WithClass(class).Do(ctx)
	if err != nil {
		return fmt.Errorf("创建 KnowledgeChunk Class 失败: %w", err)
	}
	log.Println("[Weaviate] KnowledgeChunk Class 创建成功")
	return nil
}

func (c *WeaviateClient) BatchInsert(ctx context.Context, chunks []Chunk) error {
	objects := make([]*models.Object, 0, len(chunks))
	for _, ch := range chunks {
		obj := &models.Object{
			Class: KnowledgeChunkClass,
			Properties: map[string]interface{}{
				"content":     ch.Content,
				"kb_id":       ch.KBID,
				"source_file": ch.SourceFile,
				"chunk_index": ch.ChunkIndex,
				"doc_title":   ch.DocTitle,
				"token_count": ch.TokenCount,
			},
		}
		if len(ch.Vector) > 0 {
			vec := make(models.C11yVector, len(ch.Vector))
			copy(vec, ch.Vector)
			obj.Vector = vec
		}
		objects = append(objects, obj)
	}

	batchRes, err := c.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
	if err != nil {
		return fmt.Errorf("批量写入失败: %w", err)
	}

	for _, res := range batchRes {
		if res.Result.Errors != nil {
			return fmt.Errorf("批量写入部分失败: %v", res.Result.Errors)
		}
	}
	return nil
}

func (c *WeaviateClient) HybridSearch(ctx context.Context, query string, queryVector []float32, kbIDs []string, topK int, alpha float64) ([]SearchResult, error) {
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "kb_id"},
		{Name: "source_file"},
		{Name: "chunk_index"},
		{Name: "doc_title"},
		{Name: "token_count"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "score"},
		}},
	}

	hybrid := &graphql.HybridArgumentBuilder{}
	hybrid.WithQuery(query).WithAlpha(float32(alpha))
	if len(queryVector) > 0 {
		vec := make(models.C11yVector, len(queryVector))
		copy(vec, queryVector)
		hybrid.WithVector(vec)
	}
	builder := c.client.GraphQL().Get().
		WithClassName(KnowledgeChunkClass).
		WithFields(fields...).
		WithHybrid(hybrid)

	if len(kbIDs) > 0 {
		where := filters.Where().
			WithPath([]string{"kb_id"}).
			WithOperator(filters.ContainsAny).
			WithValueString(kbIDs...)
		builder = builder.WithWhere(where)
	}

	if topK > 0 {
		builder = builder.WithLimit(topK)
	} else {
		builder = builder.WithLimit(5)
	}

	res, err := builder.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("Hybrid 检索失败: %w", err)
	}

	if res.Errors != nil {
		return nil, fmt.Errorf("GraphQL 错误: %v", res.Errors)
	}

	data := res.Data
	get, ok := data["Get"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	chunksRaw, ok := get[KnowledgeChunkClass].([]interface{})
	if !ok {
		return nil, nil
	}

	results := make([]SearchResult, 0, len(chunksRaw))
	for _, raw := range chunksRaw {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		r := SearchResult{}
		if v, ok := item["content"].(string); ok {
			r.Content = v
		}
		if v, ok := item["kb_id"].(string); ok {
			r.KBID = v
		}
		if v, ok := item["source_file"].(string); ok {
			r.SourceFile = v
		}
		if v, ok := item["chunk_index"].(float64); ok {
			r.ChunkIndex = int(v)
		}
		if v, ok := item["doc_title"].(string); ok {
			r.DocTitle = v
		}
		if additional, ok := item["_additional"].(map[string]interface{}); ok {
			if score, ok := additional["score"].(float64); ok {
				r.Score = score
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func (c *WeaviateClient) DeleteByKBID(ctx context.Context, kbID string) error {
	where := filters.Where().
		WithPath([]string{"kb_id"}).
		WithOperator(filters.Equal).
		WithValueString(kbID)

	_, err := c.client.Batch().ObjectsBatchDeleter().
		WithClassName(KnowledgeChunkClass).
		WithWhere(where).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("删除知识库 chunk 失败: %w", err)
	}
	return nil
}

func (c *WeaviateClient) GetCollectionStats(ctx context.Context, kbID string) (int, error) {
	fields := []graphql.Field{
		{Name: "meta", Fields: []graphql.Field{
			{Name: "count"},
		}},
	}

	builder := c.client.GraphQL().Aggregate().
		WithClassName(KnowledgeChunkClass).
		WithFields(fields...)

	if kbID != "" {
		where := filters.Where().
			WithPath([]string{"kb_id"}).
			WithOperator(filters.Equal).
			WithValueString(kbID)
		builder = builder.WithWhere(where)
	}

	res, err := builder.Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("获取统计数据失败: %w", err)
	}

	data := res.Data
	aggregate, ok := data["Aggregate"].(map[string]interface{})
	if !ok {
		return 0, nil
	}
	chunksRaw, ok := aggregate[KnowledgeChunkClass].([]interface{})
	if !ok || len(chunksRaw) == 0 {
		return 0, nil
	}
	item, ok := chunksRaw[0].(map[string]interface{})
	if !ok {
		return 0, nil
	}
	meta, ok := item["meta"].(map[string]interface{})
	if !ok {
		return 0, nil
	}
	count, ok := meta["count"].(float64)
	if !ok {
		return 0, nil
	}
	return int(count), nil
}
