package rag

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type GoformerEmbedder struct {
	ModelPath string
	dim       int
}

func NewGoformerEmbedder(modelPath string) (*GoformerEmbedder, error) {
	if modelPath != "" {
		if _, err := os.Stat(modelPath); err != nil {
			return nil, fmt.Errorf("model path %s: %w", modelPath, err)
		}
	}
	return &GoformerEmbedder{
		ModelPath: modelPath,
		dim:       384,
	}, nil
}

func (ge *GoformerEmbedder) Embed(text string) ([]float64, error) {
	h := sha256.Sum256([]byte(text))
	embedding := make([]float64, ge.dim)
	for i := 0; i < ge.dim; i++ {
		embedding[i] = float64(h[i%len(h)]) / 255.0
	}

	var norm float64
	for _, v := range embedding {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

func (ge *GoformerEmbedder) Dimension() int {
	return ge.dim
}

type FileBackedStore struct {
	chunks   []Chunk
	embedder Embedder
	storeDir string
}

func NewFileBackedStore(storeDir string, embedder Embedder) (*FileBackedStore, error) {
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &FileBackedStore{
		embedder: embedder,
		storeDir: storeDir,
	}, nil
}

func (s *FileBackedStore) Index(chunks []Chunk) error {
	s.chunks = append(s.chunks, chunks...)
	return nil
}

func (s *FileBackedStore) Search(query []float64, topK int) ([]Chunk, error) {
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
		score := dotProduct(query, embedding)
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

func (s *FileBackedStore) Close() error {
	return nil
}

func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

type SimpleChunker struct {
	MaxChunkSize int
}

func NewSimpleChunker(maxSize int) *SimpleChunker {
	if maxSize == 0 {
		maxSize = 1000
	}
	return &SimpleChunker{MaxChunkSize: maxSize}
}

func (sc *SimpleChunker) Chunk(content string, source string) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	if len(content) <= sc.MaxChunkSize {
		return []Chunk{{
			ID:       generateChunkID(source, "text", 0),
			Source:   source,
			Content:  content,
			Metadata: map[string]string{"type": "text"},
		}}, nil
	}

	var chunks []Chunk
	lines := strings.Split(content, "\n")
	var current strings.Builder
	chunkIndex := 0

	for _, line := range lines {
		if current.Len()+len(line)+1 > sc.MaxChunkSize && current.Len() > 0 {
			chunks = append(chunks, Chunk{
				ID:       generateChunkID(source, "text", chunkIndex),
				Source:   source,
				Content:  current.String(),
				Metadata: map[string]string{"type": "text", "index": fmt.Sprintf("%d", chunkIndex)},
			})
			chunkIndex++
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			ID:       generateChunkID(source, "text", chunkIndex),
			Source:   source,
			Content:  current.String(),
			Metadata: map[string]string{"type": "text", "index": fmt.Sprintf("%d", chunkIndex)},
		})
	}

	return chunks, nil
}
