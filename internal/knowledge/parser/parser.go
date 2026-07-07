package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ParseResult struct {
	Content  string
	DocTitle string
	IsCSV    bool
}

type Parser interface {
	Parse(filePath string) (*ParseResult, error)
}

func GetParser(ext string) (Parser, error) {
	switch strings.ToLower(ext) {
	case ".md":
		return &MarkdownParser{}, nil
	case ".pdf":
		return &PDFParser{}, nil
	case ".docx":
		return &DOCXParser{}, nil
	case ".txt":
		return &TXTParser{}, nil
	case ".csv":
		return &CSVParser{}, nil
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}
}

func ParseFile(filePath string) (*ParseResult, error) {
	ext := filepath.Ext(filePath)
	if ext == "" {
		return nil, fmt.Errorf("无法确定文件扩展名: %s", filePath)
	}
	p, err := GetParser(ext)
	if err != nil {
		return nil, err
	}
	return p.Parse(filePath)
}
