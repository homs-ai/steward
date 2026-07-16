package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Runner struct {
	Dir string
}

func (r *Runner) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r *Runner) IsRepo() bool {
	_, err := r.run("rev-parse", "--git-dir")
	return err == nil
}

func (r *Runner) IsDirty() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func (r *Runner) DefaultBranch() (string, error) {
	out, err := r.run("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		branch := strings.TrimPrefix(out, "refs/remotes/origin/")
		if branch != "" {
			return branch, nil
		}
	}

	for _, name := range []string{"main", "master", "develop"} {
		_, err := r.run("show-ref", "--verify", "refs/heads/"+name)
		if err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("cannot determine default branch")
}

func (r *Runner) CurrentBranch() (string, error) {
	return r.run("branch", "--show-current")
}

func (r *Runner) IsDetachedHead() (bool, error) {
	out, err := r.run("branch", "--show-current")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

func (r *Runner) SwitchToBranch(name string) error {
	_, err := r.run("switch", name)
	return err
}

func (r *Runner) CreateAndSwitchBranch(name string) error {
	_, err := r.run("switch", "-c", name)
	return err
}

func (r *Runner) BranchExists(name string) (bool, error) {
	_, err := r.run("show-ref", "--verify", "refs/heads/"+name)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (r *Runner) StashPush(message string) error {
	_, err := r.run("stash", "push", "-m", message)
	return err
}

// Worktree represents one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	HEAD     string
	Branch   string
	Bare     bool
	Detached bool
	Locked   bool
	Prunable bool
}

// IsBare reports whether the repository is a bare repository.
func (r *Runner) IsBare() bool {
	out, err := r.run("rev-parse", "--is-bare-repository")
	if err != nil {
		return false
	}
	return out == "true"
}

// CommonDir resolves the canonical, absolute path to the repository's common
// git directory. It uses the resolve-and-canonicalize recipe: git may report a
// relative path (e.g. ".git" or "../../.git"), which we join against the runner
// dir and then canonicalize via EvalSymlinks so that the value is invariant
// across the main worktree, secondary worktrees, and subdirectories.
func (r *Runner) CommonDir() (string, error) {
	common, err := r.run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	abs := common
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.Dir, common)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

// WorktreeList parses `git worktree list --porcelain` into a slice of Worktree.
// Entries are separated by blank lines; detached and bare entries lack a
// `branch` line, which is handled gracefully.
func (r *Runner) WorktreeList() ([]Worktree, error) {
	out, err := r.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			worktrees = append(worktrees, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		key := line
		value := ""
		if idx := strings.IndexByte(line, ' '); idx != -1 {
			key = line[:idx]
			value = line[idx+1:]
		}
		switch key {
		case "worktree":
			flush()
			cur = &Worktree{Path: value}
		case "HEAD":
			if cur != nil {
				cur.HEAD = value
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		}
	}
	flush()

	return worktrees, nil
}

// WorktreeAdd creates a new worktree at path checked out to branch. When
// newBranch is true, a new branch is created with `-b`; otherwise the existing
// branch is checked out.
func (r *Runner) WorktreeAdd(path, branch string, newBranch bool) error {
	var args []string
	if newBranch {
		args = []string{"worktree", "add", "-b", branch, path}
	} else {
		args = []string{"worktree", "add", path, branch}
	}
	_, err := r.run(args...)
	return err
}

// WorktreeRemove removes the worktree at path. It does not pass --force, so git
// refuses to remove a worktree with a dirty or untracked working tree.
func (r *Runner) WorktreeRemove(path string) error {
	_, err := r.run("worktree", "remove", path)
	return err
}

// WorktreePrune prunes worktree administrative files for worktrees whose
// directories have been deleted.
func (r *Runner) WorktreePrune() error {
	_, err := r.run("worktree", "prune")
	return err
}
