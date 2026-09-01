package rag

type Chunk struct {
	ID       string            `json:"id"`
	Source   string            `json:"source"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Embedder interface {
	Embed(text string) ([]float64, error)
	Dimension() int
}

type Chunker interface {
	Chunk(content string, source string) ([]Chunk, error)
}

type Store interface {
	Index(chunks []Chunk) error
	Search(query []float64, topK int) ([]Chunk, error)
	Close() error
}
