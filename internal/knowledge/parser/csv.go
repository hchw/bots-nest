package parser

import (
	"encoding/csv"
	"os"
	"strings"
)

type CSVParser struct{}

func (p *CSVParser) Parse(filePath string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, record := range records {
		line := strings.Join(record, ",")
		lines = append(lines, line)
	}

	return &ParseResult{
		Content: strings.Join(lines, "\n"),
		IsCSV:   true,
	}, nil
}
