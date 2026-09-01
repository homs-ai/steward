package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/k/steward/internal/rag"
)

var ragBackend string

var ragCmd = &cobra.Command{
	Use:   "rag",
	Short: "Manage RAG (Retrieval-Augmented Generation) index",
	Long:  `Set up, index, and query the build-specific RAG index.`,
}

var ragSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up RAG backend and model",
	Long:  `Download or configure the RAG model and backend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRagSetup()
	},
}

var ragInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show RAG configuration and model status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRagInfo()
	},
}

var ragIndexCmd = &cobra.Command{
	Use:   "index <collection> <source-path>",
	Short: "Index files for RAG",
	Long:  `Index build manifests and project files for retrieval.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRagIndex(args[0], args[1])
	},
}

var ragQueryCmd = &cobra.Command{
	Use:   "query <collection> <query>",
	Short: "Query the RAG index",
	Long:  `Search the indexed content for relevant information.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRagQuery(args[0], args[1])
	},
}

func runRagSetup() error {
	modelPath := cfg.RAG.ModelPath
	if modelPath == "" {
		modelPath = filepath.Join(cfg.StewardHome, "models", "goformer")
	}

	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		return fmt.Errorf("create model dir: %w", err)
	}

	if _, err := os.Stat(modelPath); err == nil {
		fmt.Printf("Model already exists at %s\n", modelPath)
		return nil
	}

	fmt.Printf("Setting up RAG backend: %s\n", cfg.RAG.Backend)
	fmt.Printf("Model path: %s\n", modelPath)
	fmt.Println("Model provisioning complete (using mock/embedded weights)")

	if cfg.RAG.StorePath != "" {
		if err := os.MkdirAll(cfg.RAG.StorePath, 0755); err != nil {
			return fmt.Errorf("create store dir: %w", err)
		}
	}

	return nil
}

func runRagInfo() error {
	fmt.Println("RAG Configuration:")
	fmt.Printf("  Backend:   %s\n", cfg.RAG.Backend)
	fmt.Printf("  Model:     %s\n", cfg.RAG.ModelPath)
	fmt.Printf("  Store:     %s\n", cfg.RAG.StorePath)
	fmt.Printf("  Enabled:   %v\n", cfg.RAG.Enabled)

	if _, err := os.Stat(cfg.RAG.ModelPath); err == nil {
		fmt.Println("  Model:     present")
		checksum, err := fileChecksum(cfg.RAG.ModelPath)
		if err == nil {
			fmt.Printf("  Checksum:  %s\n", checksum)
		}
	} else {
		fmt.Println("  Model:     not found")
	}

	storeSize := 0
	if cfg.RAG.StorePath != "" {
		filepath.Walk(cfg.RAG.StorePath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				storeSize += int(info.Size())
			}
			return nil
		})
	}
	fmt.Printf("  Index:     %d bytes\n", storeSize)

	backends := rag.SupportedBackends()
	fmt.Printf("  Supported: %s\n", strings.Join(backends, ", "))

	return nil
}

func runRagIndex(collection, sourcePath string) error {
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source path: %w", err)
	}

	fmt.Printf("Indexing collection '%s' from %s\n", collection, sourcePath)

	embedder, err := rag.NewBackend(cfg.RAG.Backend, cfg.RAG.ModelPath)
	if err != nil {
		return fmt.Errorf("create backend: %w", err)
	}

	storePath := filepath.Join(cfg.RAG.StorePath, collection)
	store, err := rag.NewFileBackedStore(storePath, embedder)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer store.Close()

	chunker := rag.NewSimpleChunker(1000)
	ragInstance := rag.NewRAG(embedder, store, chunker)

	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(sourcePath, path)
		if err := ragInstance.IndexCollection(collection, string(data), relPath); err != nil {
			fmt.Printf("  Warning: failed to index %s: %v\n", relPath, err)
		} else {
			fmt.Printf("  Indexed: %s\n", relPath)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walk source: %w", err)
	}

	fmt.Printf("Indexing complete for collection '%s'\n", collection)
	return nil
}

func runRagQuery(collection, query string) error {
	embedder, err := rag.NewBackend(cfg.RAG.Backend, cfg.RAG.ModelPath)
	if err != nil {
		return fmt.Errorf("create backend: %w", err)
	}

	storePath := filepath.Join(cfg.RAG.StorePath, collection)
	store, err := rag.NewFileBackedStore(storePath, embedder)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	defer store.Close()

	_ = time.Now()

	chunker := rag.NewSimpleChunker(1000)
	ragInstance := rag.NewRAG(embedder, store, chunker)

	results, err := ragInstance.Retrieve(query, 5)
	if err != nil {
		return fmt.Errorf("retrieve: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Results for query: %s\n\n", query)
	for i, chunk := range results {
		fmt.Printf("%d. [%s]\n%s\n\n", i+1, chunk.Source, truncateString(chunk.Content, 200))
	}

	return nil
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func init() {
	ragCmd.AddCommand(ragSetupCmd)
	ragCmd.AddCommand(ragInfoCmd)
	ragCmd.AddCommand(ragIndexCmd)
	ragCmd.AddCommand(ragQueryCmd)

	ragCmd.Flags().StringVar(&ragBackend, "backend", "goformer", "RAG backend to use")
	rootCmd.AddCommand(ragCmd)
}
