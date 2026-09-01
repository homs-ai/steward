package rag

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

type StructuredChunker struct{}

func NewStructuredChunker() *StructuredChunker {
	return &StructuredChunker{}
}

func (sc *StructuredChunker) Chunk(content string, source string) ([]Chunk, error) {
	var chunks []Chunk

	if isJSON(content) {
		chunks = append(chunks, Chunk{
			ID:       generateChunkID(source, "json", 0),
			Source:   source,
			Content:  content,
			Metadata: map[string]string{"type": "json"},
		})
		return chunks, nil
	}

	if isYAML(content) {
		sections := splitYAML(content)
		for i, section := range sections {
			chunks = append(chunks, Chunk{
				ID:       generateChunkID(source, "yaml", i),
				Source:   source,
				Content:  section,
				Metadata: map[string]string{"type": "yaml", "index": fmt.Sprintf("%d", i)},
			})
		}
		return chunks, nil
	}

	chunks = append(chunks, Chunk{
		ID:       generateChunkID(source, "text", 0),
		Source:   source,
		Content:  content,
		Metadata: map[string]string{"type": "text"},
	})

	return chunks, nil
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func isYAML(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "---") || strings.Contains(s, ":\n") || strings.Contains(s, ": ")
}

func splitYAML(content string) []string {
	parts := strings.Split(content, "\n---\n")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		parts = strings.Split(content, "---")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	if len(result) == 0 {
		return []string{content}
	}
	return result
}

func generateChunkID(source, chunkType string, index int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", source, chunkType, index)))
	return fmt.Sprintf("%x", h[:8])
}
