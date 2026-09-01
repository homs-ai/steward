package rag

import (
	"fmt"
	"sort"
	"strings"
)

type BackendConfig struct {
	Name      string
	Dimension int
	Build     func(modelPath string) (Embedder, error)
}

var registry = map[string]*BackendConfig{}

func Register(name string, config *BackendConfig) {
	registry[name] = config
}

func NewBackend(name, modelPath string) (Embedder, error) {
	config, ok := registry[name]
	if !ok {
		var supported []string
		for k := range registry {
			supported = append(supported, k)
		}
		sort.Strings(supported)
		return nil, fmt.Errorf("unknown RAG backend %q (supported: %s)", name, strings.Join(supported, ", "))
	}
	return config.Build(modelPath)
}

func SupportedBackends() []string {
	var names []string
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func init() {
	Register("goformer", &BackendConfig{
		Name:      "goformer",
		Dimension: 384,
		Build: func(modelPath string) (Embedder, error) {
			return NewGoformerEmbedder(modelPath)
		},
	})
}
