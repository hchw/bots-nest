package parser

import (
	"bufio"
	"os"
	"strings"
)

type TXTParser struct{}

func (p *TXTParser) Parse(filePath string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var text strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text.WriteString(scanner.Text())
		text.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &ParseResult{
		Content: strings.TrimSpace(text.String()),
	}, nil
}
