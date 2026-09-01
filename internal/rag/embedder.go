package rag

import (
	"fmt"
	"sort"
	"strings"
)

type TextChunker struct {
	MaxChunkSize int
	OverlapSize  int
}

func NewTextChunker(maxSize, overlap int) *TextChunker {
	if maxSize == 0 {
		maxSize = 1000
	}
	if overlap == 0 {
		overlap = 100
	}
	return &TextChunker{
		MaxChunkSize: maxSize,
		OverlapSize:  overlap,
	}
}

func (tc *TextChunker) Chunk(content string, source string) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	if len(content) <= tc.MaxChunkSize {
		return []Chunk{{
			ID:       generateChunkID(source, "text", 0),
			Source:   source,
			Content:  content,
			Metadata: map[string]string{"type": "text"},
		}}, nil
	}

	var chunks []Chunk
	lines := strings.Split(content, "\n")

	currentChunk := ""
	chunkIndex := 0

	for _, line := range lines {
		if len(currentChunk)+len(line)+1 > tc.MaxChunkSize && currentChunk != "" {
			chunks = append(chunks, Chunk{
				ID:       generateChunkID(source, "text", chunkIndex),
				Source:   source,
				Content:  strings.TrimSpace(currentChunk),
				Metadata: map[string]string{"type": "text", "index": fmt.Sprintf("%d", chunkIndex)},
			})
			chunkIndex++

			if tc.OverlapSize > 0 && len(currentChunk) > tc.OverlapSize {
				currentChunk = currentChunk[len(currentChunk)-tc.OverlapSize:]
			} else {
				currentChunk = ""
			}
		}
		if currentChunk != "" {
			currentChunk += "\n"
		}
		currentChunk += line
	}

	if strings.TrimSpace(currentChunk) != "" {
		chunks = append(chunks, Chunk{
			ID:       generateChunkID(source, "text", chunkIndex),
			Source:   source,
			Content:  strings.TrimSpace(currentChunk),
			Metadata: map[string]string{"type": "text", "index": fmt.Sprintf("%d", chunkIndex)},
		})
	}

	return chunks, nil
}

type InMemoryStore struct {
	chunks   []Chunk
	embedder Embedder
}

func NewInMemoryStore(embedder Embedder) *InMemoryStore {
	return &InMemoryStore{
		embedder: embedder,
	}
}

func (s *InMemoryStore) Index(chunks []Chunk) error {
	s.chunks = append(s.chunks, chunks...)
	return nil
}

func (s *InMemoryStore) Search(query []float64, topK int) ([]Chunk, error) {
	type scored struct {
		chunk Chunk
		score float64
	}

	var results []scored
	for _, chunk := range s.chunks {
		embedding, err := s.embedder.Embed(chunk.Content)
		if err != nil {
			continue
		}
		score := cosineSimilarity(query, embedding)
		results = append(results, scored{chunk: chunk, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	var chunks []Chunk
	for _, r := range results[:topK] {
		chunks = append(chunks, r.chunk)
	}
	return chunks, nil
}

func (s *InMemoryStore) Close() error {
	return nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

type RAG struct {
	embedder Embedder
	store    Store
	chunker  Chunker
}

func NewRAG(embedder Embedder, store Store, chunker Chunker) *RAG {
	return &RAG{
		embedder: embedder,
		store:    store,
		chunker:  chunker,
	}
}

func (r *RAG) IndexCollection(name string, content string, source string) error {
	chunks, err := r.chunker.Chunk(content, source)
	if err != nil {
		return fmt.Errorf("chunk content: %w", err)
	}
	return r.store.Index(chunks)
}

func (r *RAG) Retrieve(query string, topK int) ([]Chunk, error) {
	embedding, err := r.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	return r.store.Search(embedding, topK)
}
