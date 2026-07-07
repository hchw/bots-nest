package parser

import (
	"strings"

	"github.com/nguyenthenguyen/docx"
)

type DOCXParser struct{}

func (p *DOCXParser) Parse(filePath string) (*ParseResult, error) {
	r, err := docx.ReadDocxFile(filePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	content := r.Editable().GetContent()
	return &ParseResult{
		Content: strings.TrimSpace(content),
	}, nil
}
