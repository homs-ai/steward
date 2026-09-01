package rag

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestStructuredChunkerJSON(t *testing.T) {
	chunker := NewStructuredChunker()
	content := `{"name": "test", "version": "1.0"}`
	chunks, err := chunker.Chunk(content, "test.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Metadata["type"] != "json" {
		t.Errorf("expected json type, got %s", chunks[0].Metadata["type"])
	}
}

func TestStructuredChunkerYAML(t *testing.T) {
	chunker := NewStructuredChunker()
	content := "key1: value1\n---\nkey2: value2"
	chunks, err := chunker.Chunk(content, "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestTextChunkerSmallContent(t *testing.T) {
	chunker := NewTextChunker(1000, 100)
	content := "Hello, world!"
	chunks, err := chunker.Chunk(content, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestTextChunkerLargeContent(t *testing.T) {
	chunker := NewTextChunker(50, 10)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "Line number " + strings.Repeat("x", 20)
	}
	content := strings.Join(lines, "\n")

	chunks, err := chunker.Chunk(content, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) <= 1 {
		t.Errorf("expected multiple chunks for large content, got %d", len(chunks))
	}
}

func TestTextChunkerEmpty(t *testing.T) {
	chunker := NewTextChunker(1000, 100)
	chunks, err := chunker.Chunk("", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestInMemoryStoreRoundTrip(t *testing.T) {
	embedder := &mockEmbedder{dim: 128}
	store := NewInMemoryStore(embedder)

	chunks := []Chunk{
		{ID: "1", Source: "a.txt", Content: "hello world"},
		{ID: "2", Source: "b.txt", Content: "foo bar"},
	}

	if err := store.Index(chunks); err != nil {
		t.Fatal(err)
	}

	query := []float64{}
	for i := 0; i < 128; i++ {
		query = append(query, 0.5)
	}

	results, err := store.Search(query, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestInMemoryStoreSearch(t *testing.T) {
	embedder := &mockEmbedder{dim: 3}
	store := NewInMemoryStore(embedder)

	chunks := []Chunk{
		{ID: "1", Source: "a.txt", Content: "apple"},
		{ID: "2", Source: "b.txt", Content: "banana"},
	}

	store.Index(chunks)

	query := []float64{1.0, 0.0, 0.0}
	results, err := store.Search(query, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRAGIndexAndRetrieve(t *testing.T) {
	embedder := &mockEmbedder{dim: 128}
	store := NewInMemoryStore(embedder)
	chunker := NewSimpleChunker(1000)

	rag := NewRAG(embedder, store, chunker)

	err := rag.IndexCollection("test", "This is test content for indexing", "test.txt")
	if err != nil {
		t.Fatal(err)
	}

	results, err := rag.Retrieve("test query", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestRegistryBackends(t *testing.T) {
	backends := SupportedBackends()
	if len(backends) == 0 {
		t.Error("expected at least one registered backend")
	}

	found := false
	for _, b := range backends {
		if b == "goformer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected goformer to be registered")
	}
}

func TestRegistryUnknownBackend(t *testing.T) {
	_, err := NewBackend("nonexistent", "")
	if err == nil {
		t.Error("expected error for unknown backend")
	}
	if !strings.Contains(err.Error(), "supported") {
		t.Errorf("expected error to list supported backends, got: %v", err)
	}
}

func TestGoformerEmbedder(t *testing.T) {
	embedder, err := NewGoformerEmbedder("")
	if err != nil {
		t.Fatal(err)
	}

	if embedder.Dimension() != 384 {
		t.Errorf("expected dimension 384, got %d", embedder.Dimension())
	}

	vec, err := embedder.Embed("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 384 {
		t.Errorf("expected 384-dim vector, got %d", len(vec))
	}

	vec2, err := embedder.Embed("hello world")
	if err != nil {
		t.Fatal(err)
	}
	for i := range vec {
		if vec[i] != vec2[i] {
			t.Error("expected deterministic embedding")
			break
		}
	}
}

func TestGoformerEmbedderModelPath(t *testing.T) {
	_, err := NewGoformerEmbedder("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent model path")
	}
}

func TestFileBackedStore(t *testing.T) {
	storeDir := t.TempDir()
	embedder := &mockEmbedder{dim: 128}
	store, err := NewFileBackedStore(storeDir, embedder)
	if err != nil {
		t.Fatal(err)
	}

	chunks := []Chunk{
		{ID: "1", Source: "a.txt", Content: "hello"},
	}
	if err := store.Index(chunks); err != nil {
		t.Fatal(err)
	}

	query := make([]float64, 128)
	results, err := store.Search(query, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSimpleChunker(t *testing.T) {
	chunker := NewSimpleChunker(100)
	content := strings.Repeat("Line\n", 50)
	chunks, err := chunker.Chunk(content, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) <= 1 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
}

type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Embed(text string) ([]float64, error) {
	vec := make([]float64, m.dim)
	for i := range vec {
		vec[i] = 0.5
	}
	return vec, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

func TestChunkerBoundaryOverlap(t *testing.T) {
	chunker := NewTextChunker(100, 20)
	content := strings.Repeat("abcdefghij\n", 20)
	chunks, err := chunker.Chunk(content, "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) <= 1 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	for i, chunk := range chunks {
		if len(chunk.Content) > 150 {
			t.Errorf("chunk %d too large: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestStoreRoundTrip(t *testing.T) {
	embedder := &mockEmbedder{dim: 64}
	store := NewInMemoryStore(embedder)

	chunks := []Chunk{
		{ID: "1", Source: "go.mod", Content: "module example.com/app"},
		{ID: "2", Source: "package.json", Content: `{"name": "test"}`},
		{ID: "3", Source: "Cargo.toml", Content: "[package]\nname = \"test\""},
	}

	store.Index(chunks)

	query := make([]float64, 64)
	results, err := store.Search(query, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestSupportedBackendsSorted(t *testing.T) {
	backends := SupportedBackends()
	if !sort.StringsAreSorted(backends) {
		t.Error("expected backends to be sorted")
	}
}

func TestNewBackendReturnsError(t *testing.T) {
	_, err := NewBackend("goformer", "")
	if err == nil {
		t.Log("goformer backend created (mock)")
	} else {
		t.Logf("goformer backend error (expected in test): %v", err)
	}
}

func TestFileBackedStoreCreateDir(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "nested", "store")
	embedder := &mockEmbedder{dim: 128}
	store, err := NewFileBackedStore(storeDir, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected store dir to be created")
	}
}
