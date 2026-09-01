package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFingerprintEmpty(t *testing.T) {
	fp, err := ComputeFingerprint(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp == "" {
		t.Error("expected non-empty fingerprint")
	}
}

func TestComputeFingerprintStable(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0644)

	fp1, err := ComputeFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := ComputeFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprint not stable: %s != %s", fp1, fp2)
	}
}

func TestComputeFingerprintChangesOnManifestEdit(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0644)
	fp1, _ := ComputeFingerprint(root)

	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0644)
	fp2, _ := ComputeFingerprint(root)

	if fp1 == fp2 {
		t.Error("expected fingerprint to change after manifest edit")
	}
}

func TestFingerprintChanged(t *testing.T) {
	root := t.TempDir()
	fp, _ := ComputeFingerprint(root)

	changed, err := FingerprintChanged(root, fp)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected unchanged for same state")
	}

	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0644)
	changed, err = FingerprintChanged(root, fp)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed after adding go.mod")
	}
}
