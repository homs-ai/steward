package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/k/steward/internal/git"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")
	return dir
}

func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

func TestRunGitBranchSetupFromCurrent(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	base, err := runGitBranchSetup(gr, branchFromCurrent, "test-feature", "feat/test-feature")
	if err != nil {
		t.Fatalf("runGitBranchSetup() returned error: %v", err)
	}

	current, _ := gr.CurrentBranch()
	if current != "feat/test-feature" {
		t.Errorf("expected current branch %q, got %q", "feat/test-feature", current)
	}

	if base == "" {
		t.Error("expected non-empty base branch")
	}

	exists, _ := gr.BranchExists("feat/test-feature")
	if !exists {
		t.Error("expected branch to exist after creation")
	}
}

func TestRunGitBranchSetupFromDefault(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	base, err := runGitBranchSetup(gr, branchFromDefault, "test-feature", "feat/test-feature")
	if err != nil {
		t.Fatalf("runGitBranchSetup() returned error: %v", err)
	}

	current, _ := gr.CurrentBranch()
	if current != "feat/test-feature" {
		t.Errorf("expected current branch %q, got %q", "feat/test-feature", current)
	}

	if base == "" {
		t.Error("expected non-empty base branch")
	}

	exists, _ := gr.BranchExists("feat/test-feature")
	if !exists {
		t.Error("expected branch to exist after creation")
	}
}

func TestRunGitBranchSetupDuplicate(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	_, err := runGitBranchSetup(gr, branchFromCurrent, "test", "existing-branch")
	if err != nil {
		t.Fatalf("first runGitBranchSetup() returned error: %v", err)
	}

	runGit(t, dir, "switch", "-")

	_, err = runGitBranchSetup(gr, branchFromCurrent, "test", "existing-branch")
	if err == nil {
		t.Fatal("expected error for duplicate branch name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got %q", err.Error())
	}
}

func TestRunGitBranchSetupSkip(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	currentBefore, _ := gr.CurrentBranch()

	base, err := runGitBranchSetup(gr, branchSkip, "test", "should-not-create")
	if err != nil {
		t.Fatalf("runGitBranchSetup(skip) returned error: %v", err)
	}
	if base != "" {
		t.Errorf("expected empty base for skip, got %q", base)
	}

	currentAfter, _ := gr.CurrentBranch()
	if currentAfter != currentBefore {
		t.Errorf("expected branch unchanged after skip, got %q", currentAfter)
	}

	exists, _ := gr.BranchExists("should-not-create")
	if exists {
		t.Error("expected branch not to exist after skip")
	}
}

func TestPromptBranchChoiceFromCurrent(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("1\n"))
	w.Close()

	action := promptBranchChoice(gr, "test-feature", "feat/test-feature")
	if action != branchFromCurrent {
		t.Errorf("expected branchFromCurrent, got %v", action)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestPromptBranchChoiceSkip(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	gr := &git.Runner{Dir: dir}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	// Send invalid option to test fallback to skip
	w.Write([]byte("0\n"))
	w.Close()

	action := promptBranchChoice(gr, "test-feature", "feat/test-feature")
	if action != branchSkip {
		t.Errorf("expected branchSkip for option 3, got %v", action)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestPromptBranchChoiceDetachedHead(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	runGit(t, dir, "checkout", "--detach")
	gr := &git.Runner{Dir: dir}

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("2\n"))
	w.Close()

	action := promptBranchChoice(gr, "test-feature", "feat/test-feature")
	if action != branchSkip {
		t.Errorf("expected branchSkip for option 2 in detached head, got %v", action)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}
