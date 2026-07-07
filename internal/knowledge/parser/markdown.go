package parser

import (
	"os"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
)

type MarkdownParser struct{}

func (p *MarkdownParser) Parse(filePath string) (*ParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	doc := markdown.Parse(data, nil)
	var title string
	var text strings.Builder

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext
		}
		switch n := node.(type) {
		case *ast.Heading:
			if n.Level == 1 && title == "" {
				title = extractText(n)
			}
		case *ast.Text:
			text.WriteString(string(n.Literal))
		case *ast.CodeBlock:
			text.WriteString(string(n.Literal))
		case *ast.Code:
			text.WriteString(string(n.Literal))
		}
		return ast.GoToNext
	})

	content := strings.TrimSpace(text.String())
	if content == "" {
		content = string(data)
	}

	return &ParseResult{
		Content:  content,
		DocTitle: title,
	}, nil
}

func extractText(node ast.Node) string {
	var buf strings.Builder
	ast.WalkFunc(node, func(n ast.Node, entering bool) ast.WalkStatus {
		if text, ok := n.(*ast.Text); ok && entering {
			buf.WriteString(string(text.Literal))
		}
		return ast.GoToNext
	})
	return strings.TrimSpace(buf.String())
}
