package knowledge

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/hchw/bots-nest/internal/config"
	"github.com/hchw/bots-nest/internal/db"
	"github.com/hchw/bots-nest/internal/knowledge/parser"
	"github.com/hchw/bots-nest/internal/knowledge/splitter"
	"github.com/hchw/bots-nest/internal/ws"
)

type ImportTaskManager struct {
	weaviate   *WeaviateClient
	embedder   *Embedder
	cfg        *config.KnowledgeBaseConfig
	hub        *ws.Hub
	mu         sync.Mutex
	storageDir string
}

func NewImportTaskManager(wc *WeaviateClient, embedder *Embedder, cfg *config.KnowledgeBaseConfig, hub *ws.Hub, storageDir string) *ImportTaskManager {
	return &ImportTaskManager{
		weaviate:   wc,
		embedder:   embedder,
		cfg:        cfg,
		hub:        hub,
		storageDir: storageDir,
	}
}

func (m *ImportTaskManager) ReceiveFile(kbID, fileName string, fileData io.Reader, fileSize int64) (*db.ImportTask, error) {
	if err := os.MkdirAll(m.storageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	ext := filepath.Ext(fileName)
	allowed := false
	for _, e := range m.cfg.AllowedExtensions {
		if e == ext {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("不支持的文件格式: %s，允许的格式: %v", ext, m.cfg.AllowedExtensions)
	}

	if fileSize > m.cfg.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制 (%d > %d)", fileSize, m.cfg.MaxFileSize)
	}

	taskID := uuid.New().String()
	storagePath := filepath.Join(m.storageDir, taskID+ext)

	dst, err := os.Create(storagePath)
	if err != nil {
		return nil, fmt.Errorf("创建存储文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileData); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	task := db.ImportTask{
		ID:              taskID,
		KnowledgeBaseID: kbID,
		FileName:        fileName,
		FilePath:        storagePath,
		FileSize:        fileSize,
		Status:          "pending",
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建导入任务失败: %w", err)
	}

	go m.processTask(&task)

	return &task, nil
}

func (m *ImportTaskManager) processTask(task *db.ImportTask) {
	ctx := context.Background()

	m.updateTaskStatus(task, "parsing")

	parseResult, err := parser.ParseFile(task.FilePath)
	if err != nil {
		m.updateTaskError(task, fmt.Sprintf("解析失败: %v", err))
		return
	}

	m.updateTaskStatus(task, "chunking")

	var chunks []Chunk
	chunkIndex := 0

	if parseResult.IsCSV {
			lines := splitIntoLines(parseResult.Content)
			task.TotalChunks = len(lines)
			db.DB.Model(task).Update("total_chunks", task.TotalChunks)

			for _, line := range lines {
			chunks = append(chunks, Chunk{
				Content:    line,
				KBID:       task.KnowledgeBaseID,
				SourceFile: task.FileName,
				ChunkIndex: chunkIndex,
				DocTitle:   task.FileName,
				TokenCount: len(line) / 4,
			})
			chunkIndex++
		}
	} else {
		sp := splitter.NewTextSplitter(m.cfg.ChunkSize, m.cfg.ChunkOverlap)
		splitChunks := sp.Split(parseResult.Content)
		task.TotalChunks = len(splitChunks)
		db.DB.Model(task).Update("total_chunks", task.TotalChunks)

		for _, chunkText := range splitChunks {
			chunks = append(chunks, Chunk{
				Content:    chunkText,
				KBID:       task.KnowledgeBaseID,
				SourceFile: task.FileName,
				ChunkIndex: chunkIndex,
				DocTitle:   parseResult.DocTitle,
				TokenCount: len(chunkText) / 4,
			})
			chunkIndex++
		}
	}

	m.updateTaskStatus(task, "indexing")

	// Look up KB's embedding config
	var kb db.KnowledgeBase
	if err := db.DB.Where("id = ?", task.KnowledgeBaseID).First(&kb).Error; err != nil {
		m.updateTaskError(task, fmt.Sprintf("知识库未找到: %v", err))
		return
	}

	// Embed all chunks
	for i := 0; i < len(chunks); i += 20 {
		end := i + 20
		if end > len(chunks) {
			end = len(chunks)
		}
		texts := make([]string, 0, end-i)
		for _, ch := range chunks[i:end] {
			texts = append(texts, ch.Content)
		}
		vectors, err := m.embedder.Embed(kb.EmbeddingProviderID, kb.EmbeddingModel, texts)
		if err != nil {
			m.updateTaskError(task, fmt.Sprintf("向量化失败: %v", err))
			return
		}
		for j, vec := range vectors {
			chunks[i+j].Vector = vec
		}
	}

	batchSize := 100
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		if err := m.weaviate.BatchInsert(ctx, chunks[i:end]); err != nil {
			m.updateTaskError(task, fmt.Sprintf("批量写入失败: %v", err))
			return
		}

		task.ProcessedChunks = end
		db.DB.Model(task).Update("processed_chunks", end)
		m.hub.BroadcastTaskProgress(task.ID, task.Status, task.TotalChunks, task.ProcessedChunks)
	}

	// Update knowledge base file count
	var count int64
	db.DB.Model(&db.ImportTask{}).
		Where("knowledge_base_id = ? AND status = 'completed'", task.KnowledgeBaseID).
		Count(&count)
	db.DB.Model(&db.KnowledgeBase{}).
		Where("id = ?", task.KnowledgeBaseID).
		Update("file_count", count)

	m.updateTaskStatus(task, "completed")
	log.Printf("[Importer] 导入任务 %s 完成: %s (%d chunks)", task.ID, task.FileName, task.TotalChunks)
}

func (m *ImportTaskManager) updateTaskStatus(task *db.ImportTask, status string) {
	task.Status = status
	db.DB.Model(task).Update("status", status)
	m.hub.BroadcastTaskProgress(task.ID, status, task.TotalChunks, task.ProcessedChunks)
}

func (m *ImportTaskManager) updateTaskError(task *db.ImportTask, errMsg string) {
	task.Status = "failed"
	task.Error = errMsg
	db.DB.Model(task).Updates(map[string]interface{}{
		"status": "failed",
		"error":  errMsg,
	})
	m.hub.BroadcastTaskProgress(task.ID, "failed", task.TotalChunks, task.ProcessedChunks)
	log.Printf("[Importer] 导入任务 %s 失败: %s", task.ID, errMsg)
}

func splitIntoLines(content string) []string {
	var lines []string
	var current string
	for _, ch := range content {
		if ch == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = append(lines, content)
	}
	return lines
}
