package knowledge

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hchw/bots-nest/internal/llm"
)

const kbSummarySystemPrompt = "你是一个知识库内容摘要助手。下面给出某个知识库中的若干文本片段（来自不同文件）。" +
	"请用简洁的中文（不超过120字）概括该知识库整体涵盖的主题与内容范围，说明它适合回答哪类问题。" +
	"只输出摘要本身，不要使用标题、列表或额外说明。"

func GenerateKBSummary(ctx context.Context, client *llm.OpenAIClient, samples []SearchResult) (string, error) {
	if len(samples) == 0 {
		return "", fmt.Errorf("没有可采样的文本片段")
	}
	log.Printf("[KB-Summary] 开始生成知识库摘要, 采样片段数=%d", len(samples))

	var buf strings.Builder
	for i, s := range samples {
		title := s.DocTitle
		if title == "" {
			title = s.SourceFile
		}
		buf.WriteString(fmt.Sprintf("[片段 %d] 来源: %s\n%s\n\n", i+1, title, s.Content))
	}

	msgs := []llm.ChatMessage{
		{Role: "system", Content: kbSummarySystemPrompt},
		{Role: "user", Content: "请概括以下知识库片段的内容主题：\n\n" + buf.String()},
	}

	log.Printf("[KB-Summary] 调用 LLM 生成摘要 (prompt 长度=%d 字符)", len(msgs[1].Content))
	resp, err := client.Chat(msgs, nil)
	if err != nil {
		return "", fmt.Errorf("生成知识库摘要失败: %w", err)
	}
	log.Printf("[KB-Summary] LLM 返回摘要原文: %q", resp.Content)

	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return "", fmt.Errorf("知识库摘要为空")
	}
	log.Printf("[Knowledge] 生成知识库摘要成功: %d 字符", len(summary))
	return summary, nil
}
