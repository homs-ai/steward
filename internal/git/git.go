package git

import (
	"bytes"
	"fmt"
	"os/exec"
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
