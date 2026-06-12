package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestIsRepo(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}
	if !r.IsRepo() {
		t.Error("IsRepo() = false, want true")
	}

	nonRepo := t.TempDir()
	r2 := &Runner{Dir: nonRepo}
	if r2.IsRepo() {
		t.Error("IsRepo() = true, want false")
	}
}

func TestIsDirty(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	dirty, err := r.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("IsDirty() = true, want false (clean after init)")
	}

	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	dirty, err = r.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Error("IsDirty() = false, want true (after adding file)")
	}
}

func TestDefaultBranch(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	branch, err := r.DefaultBranch()
	if err != nil {
		t.Fatalf("DefaultBranch() returned error: %v", err)
	}

	current, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch != current {
		t.Errorf("DefaultBranch() = %q, want CurrentBranch() = %q", branch, current)
	}
}

func TestDefaultBranchOriginHEAD(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	runGit(t, dir, "branch", "main")

	refDir := filepath.Join(dir, ".git", "refs", "remotes", "origin")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "HEAD"), []byte("ref: refs/remotes/origin/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{Dir: dir}
	branch, err := r.DefaultBranch()
	if err != nil {
		t.Fatalf("DefaultBranch() returned error: %v", err)
	}
	if branch != "main" {
		t.Errorf("DefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestDefaultBranchFail(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	runGit(t, dir, "branch", "-m", "custom-branch")

	r := &Runner{Dir: dir}
	_, err := r.DefaultBranch()
	if err == nil {
		t.Error("DefaultBranch() expected error for non-standard branch name")
	}
}

func TestCurrentBranch(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	branch, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" {
		t.Error("CurrentBranch() returned empty string")
	}

	nonRepo := t.TempDir()
	r2 := &Runner{Dir: nonRepo}
	_, err = r2.CurrentBranch()
	if err == nil {
		t.Error("CurrentBranch() expected error outside git repo")
	}
}

func TestIsDetachedHead(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	detached, err := r.IsDetachedHead()
	if err != nil {
		t.Fatal(err)
	}
	if detached {
		t.Error("IsDetachedHead() = true, want false on new branch")
	}

	runGit(t, dir, "checkout", "--detach")

	detached, err = r.IsDetachedHead()
	if err != nil {
		t.Fatal(err)
	}
	if !detached {
		t.Error("IsDetachedHead() = false, want true after checkout --detach")
	}
}

func TestSwitchToBranch(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	currentBefore, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}

	runGit(t, dir, "branch", "other-branch")

	if err := r.SwitchToBranch("other-branch"); err != nil {
		t.Fatalf("SwitchToBranch() returned error: %v", err)
	}

	currentAfter, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if currentAfter != "other-branch" {
		t.Errorf("CurrentBranch() = %q after switch, want %q", currentAfter, "other-branch")
	}

	if err := r.SwitchToBranch(currentBefore); err != nil {
		t.Fatalf("SwitchToBranch() back returned error: %v", err)
	}

	currentAfter, err = r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if currentAfter != currentBefore {
		t.Errorf("CurrentBranch() = %q after switch back, want %q", currentAfter, currentBefore)
	}
}

func TestCreateAndSwitchBranch(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	if err := r.CreateAndSwitchBranch("new-branch"); err != nil {
		t.Fatalf("CreateAndSwitchBranch() returned error: %v", err)
	}

	current, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if current != "new-branch" {
		t.Errorf("CurrentBranch() = %q, want %q", current, "new-branch")
	}

	exists, err := r.BranchExists("new-branch")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("BranchExists('new-branch') = false, want true")
	}
}

func TestCreateAndSwitchBranchCollision(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	if err := r.CreateAndSwitchBranch("existing"); err != nil {
		t.Fatalf("CreateAndSwitchBranch() returned error: %v", err)
	}

	runGit(t, dir, "switch", "-")

	if err := r.CreateAndSwitchBranch("existing"); err == nil {
		t.Error("CreateAndSwitchBranch() expected error for duplicate branch name")
	}
}

func TestBranchExists(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	current, err := r.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}

	exists, err := r.BranchExists(current)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Errorf("BranchExists(%q) = false, want true", current)
	}

	exists, err = r.BranchExists("nonexistent-branch")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("BranchExists('nonexistent-branch') = true, want false")
	}
}

func TestStashPush(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	// Create a tracked file with changes
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-m", "add tracked file")

	// Modify the tracked file
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	dirty, _ := r.IsDirty()
	if !dirty {
		t.Fatal("expected dirty working tree before stash")
	}

	if err := r.StashPush("test stash message"); err != nil {
		t.Fatalf("StashPush() returned error: %v", err)
	}

	dirty, err := r.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("expected clean working tree after stash")
	}
}

func TestStashPushCleanTree(t *testing.T) {
	skipIfNoGit(t)

	dir := newTestRepo(t)
	r := &Runner{Dir: dir}

	err := r.StashPush("stash on clean tree")
	if err != nil {
		t.Fatalf("StashPush() returned error on clean tree: %v", err)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Feature", "my-feature"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$chars", "special-chars"},
		{"spaces and dashes - test", "spaces-and-dashes-test"},
		{"...dots...", "dots"},
		{"  leading trailing  ", "leading-trailing"},
		{"-leading-hyphen", "leading-hyphen"},
		{".leading-dot", "leading-dot"},
		{"name.lock", "name"},
		{"", "feature"},
		{"valid-branch-name", "valid-branch-name"},
		{"unicode™symbols", "unicode-symbols"},
		{"with@{at}", "with-at"},
		{"feature/name", "feature/name"},
		{"CamelCaseName", "camelcasename"},
		{"under_score", "under_score"},
		{"a", "a"},
	}

	for _, tt := range tests {
		got := SanitizeBranchName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeBranchNameTooLong(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := SanitizeBranchName(long)
	if len(got) > 250 {
		t.Errorf("expected max 250 chars, got %d", len(got))
	}
	if !strings.HasPrefix(got, strings.Repeat("a", 250)) {
		t.Errorf("expected first 250 chars to be 'a'*250, got prefix %q", got[:10])
	}
}

func TestSanitizeBranchNameEmptyAfterSanitization(t *testing.T) {
	got := SanitizeBranchName("---...///")
	if got != "feature" {
		t.Errorf("SanitizeBranchName('---...///') = %q, want 'feature'", got)
	}
}
