package feature

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type FeatureMetadata struct {
	BranchName   string `yaml:"branch_name,omitempty"`
	BaseBranch   string `yaml:"base_branch,omitempty"`
	WorktreePath string `yaml:"worktree_path,omitempty"`
}

func (f *Feature) MetadataFile() string {
	return filepath.Join(f.Dir, "metadata.yaml")
}

func (f *Feature) SaveMetadata() error {
	meta := &FeatureMetadata{
		BranchName:   f.BranchName,
		BaseBranch:   f.BaseBranch,
		WorktreePath: f.WorktreePath,
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(f.MetadataFile(), data, 0644)
}

func (f *Feature) LoadMetadata() error {
	data, err := os.ReadFile(f.MetadataFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var meta FeatureMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return err
	}
	f.BranchName = meta.BranchName
	f.BaseBranch = meta.BaseBranch
	f.WorktreePath = meta.WorktreePath
	return nil
}

func (f *Feature) SetBranch(name, base string) error {
	f.BranchName = name
	f.BaseBranch = base
	return f.SaveMetadata()
}
