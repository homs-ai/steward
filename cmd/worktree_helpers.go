package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/git"
)

// repoRoot returns the top-level directory of the repository containing dir.
func repoRoot(gr *git.Runner) (string, error) {
	common, err := gr.CommonDir()
	if err != nil {
		return "", err
	}
	// common is <repo>/.git for the main worktree; its parent is the repo root.
	return filepath.Dir(common), nil
}

// worktreeSiblingPath computes the flat-sibling worktree path
// `<repo>-<sanitized-feature>` under the parent of the repository root.
func worktreeSiblingPath(root, featureName string) string {
	sanitized := git.SanitizeBranchName(featureName)
	base := filepath.Base(root)
	return filepath.Join(filepath.Dir(root), base+"-"+sanitized)
}

// branchCheckedOut reports whether branch is currently checked out in any
// worktree, returning the path of that worktree. This enforces git's
// one-branch-one-worktree rule before we attempt a `worktree add`.
func branchCheckedOut(gr *git.Runner, branch string) (bool, string, error) {
	worktrees, err := gr.WorktreeList()
	if err != nil {
		return false, "", err
	}
	for _, wt := range worktrees {
		if wt.Branch == branch {
			return true, wt.Path, nil
		}
	}
	return false, "", nil
}

// removeWorktree tears down a feature's worktree, guarding against a dirty tree
// (git worktree remove refuses without --force). On success it clears
// worktree_path from the feature metadata.
func removeWorktree(gr *git.Runner, feat *feature.Feature) error {
	path := feat.WorktreePath
	if path == "" {
		return fmt.Errorf("feature %q has no worktree", feat.Name)
	}

	if err := gr.WorktreeRemove(path); err != nil {
		return fmt.Errorf("remove worktree (has it uncommitted changes? commit or discard them, or remove manually): %w", err)
	}

	feat.WorktreePath = ""
	if err := feat.SaveMetadata(); err != nil {
		return fmt.Errorf("clear worktree_path metadata: %w", err)
	}
	return nil
}
