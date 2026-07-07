package splitter

import (
	"testing"
)

func TestSplitShortText(t *testing.T) {
	s := NewTextSplitter(500, 50)
	chunks := s.Split("hello world")
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", chunks[0])
	}
}

func TestSplitLongText(t *testing.T) {
	s := NewTextSplitter(10, 2)
	text := "aaaaa\nbbbbb\nccccc\nddddd\neeeee"
	chunks := s.Split(text)
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSplitEmptyText(t *testing.T) {
	s := NewTextSplitter(500, 50)
	chunks := s.Split("")
	if chunks != nil {
		t.Fatalf("expected nil for empty text, got %v", chunks)
	}
}

func TestSplitWithSeparators(t *testing.T) {
	s := NewTextSplitter(20, 2)
	text := "第一段。第二段。第三段。第四段"
	chunks := s.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks with Chinese separators, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > 30 {
			t.Fatalf("chunk %d too long: %d chars", i, len(chunk))
		}
	}
}
