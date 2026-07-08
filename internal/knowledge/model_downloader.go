package knowledge

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func EnsureModel(modelPath, modelURL string) error {
	if _, err := os.Stat(modelPath); err == nil {
		log.Printf("[ModelDownloader] 模型文件已存在: %s", modelPath)
		return nil
	}

	if modelURL == "" {
		log.Printf("[ModelDownloader] 未配置模型下载 URL，跳过下载")
		return nil
	}

	dir := filepath.Dir(modelPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建模型目录失败: %w", err)
	}

	log.Printf("[ModelDownloader] 开始下载模型: %s -> %s", modelURL, modelPath)
	if err := downloadFile(modelPath, modelURL); err != nil {
		return fmt.Errorf("模型下载失败: %w", err)
	}

	info, err := os.Stat(modelPath)
	if err != nil {
		return fmt.Errorf("检查下载文件失败: %w", err)
	}
	log.Printf("[ModelDownloader] 模型下载完成: %s (%d bytes)", modelPath, info.Size())
	return nil
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("下载文件为空")
	}
	return nil
}
