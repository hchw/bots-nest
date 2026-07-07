package parser

import (
	"strings"

	"github.com/ledongthuc/pdf"
)

type PDFParser struct{}

func (p *PDFParser) Parse(filePath string) (*ParseResult, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var text strings.Builder
	totalPage := r.NumPage()

	for pageNum := 1; pageNum <= totalPage; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
		text.WriteString("\n")
	}

	return &ParseResult{
		Content: strings.TrimSpace(text.String()),
	}, nil
}
