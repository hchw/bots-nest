package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTXTParser(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "这是第一行\n这是第二行\n这是第三行"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &TXTParser{}
	res, err := p.Parse(filePath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if res.Content != content {
		t.Fatalf("expected '%s', got '%s'", content, res.Content)
	}
}

func TestCSVParser(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.csv")
	content := "name,age\nAlice,30\nBob,25"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &CSVParser{}
	res, err := p.Parse(filePath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !res.IsCSV {
		t.Fatal("expected IsCSV=true")
	}
	if res.Content == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestGetParser(t *testing.T) {
	tests := []struct {
		ext      string
		wantType string
		wantErr  bool
	}{
		{".md", "*parser.MarkdownParser", false},
		{".pdf", "*parser.PDFParser", false},
		{".docx", "*parser.DOCXParser", false},
		{".txt", "*parser.TXTParser", false},
		{".csv", "*parser.CSVParser", false},
		{".jpg", "", true},
	}

	for _, tt := range tests {
		p, err := GetParser(tt.ext)
		if tt.wantErr {
			if err == nil {
				t.Errorf("GetParser(%s) expected error", tt.ext)
			}
			continue
		}
		if err != nil {
			t.Errorf("GetParser(%s) unexpected error: %v", tt.ext, err)
			continue
		}
		if p == nil {
			t.Errorf("GetParser(%s) returned nil", tt.ext)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if res.Content != "hello" {
		t.Fatalf("expected 'hello', got '%s'", res.Content)
	}
}
