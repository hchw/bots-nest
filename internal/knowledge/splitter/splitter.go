package splitter

import (
	"strings"
)

type TextSplitter struct {
	ChunkSize  int
	Overlap    int
	Separators []string
}

func NewTextSplitter(chunkSize, overlap int) *TextSplitter {
	return &TextSplitter{
		ChunkSize:  chunkSize,
		Overlap:    overlap,
		Separators: []string{"\n\n", "\n", "。", ".", " ", ""},
	}
}

func (s *TextSplitter) Split(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return s.splitText(text, s.Separators)
}

func (s *TextSplitter) splitText(text string, separators []string) []string {
	var chunks []string

	if len(separators) == 0 {
		separators = []string{""}
	}

	separator := separators[0]
	remainingSeparators := separators[1:]

	// Try to split with current separator
	parts := s.splitWithSeparator(text, separator)

	// If we got only one part and there are more separators, try with next one
	if len(parts) == 1 && len(remainingSeparators) > 0 {
		return s.splitText(text, remainingSeparators)
	}

	var currentChunk strings.Builder
	for _, part := range parts {
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(separator)
		}
		currentChunk.WriteString(part)

		if currentChunk.Len() >= s.ChunkSize || (currentChunk.Len() > 0 && len(remainingSeparators) == 0 && separator == "") {
			chunkStr := strings.TrimSpace(currentChunk.String())
			if chunkStr != "" {
				chunks = append(chunks, chunkStr)
			}
			currentChunk.Reset()

			// Add overlap
			if s.Overlap > 0 && len(part) > s.Overlap {
				overlapText := part[len(part)-s.Overlap:]
				currentChunk.WriteString(overlapText)
			}
		}
	}

	if currentChunk.Len() > 0 {
		chunkStr := strings.TrimSpace(currentChunk.String())
		if chunkStr != "" {
			chunks = append(chunks, chunkStr)
		}
	}

	// If no chunks were produced, try with next separator
	if len(chunks) == 0 && len(remainingSeparators) > 0 {
		return s.splitText(text, remainingSeparators)
	}

	// If any chunk is still too large, recursively split it
	if len(remainingSeparators) > 0 {
		var refined []string
		for _, chunk := range chunks {
			if len(chunk) > s.ChunkSize {
				subChunks := s.splitText(chunk, remainingSeparators)
				refined = append(refined, subChunks...)
			} else {
				refined = append(refined, chunk)
			}
		}
		chunks = refined
	}

	return chunks
}

func (s *TextSplitter) splitWithSeparator(text string, separator string) []string {
	if separator == "" {
		return []string{text}
	}
	parts := strings.Split(text, separator)
	// Filter out empty parts only if separator is not empty string
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{text}
	}
	return result
}
