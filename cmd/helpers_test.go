package cmd

import (
	"os"
	"testing"
)

func TestConfirmYes(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("y\n"))
	w.Close()

	result := confirm("Proceed?")
	if !result {
		t.Error("expected true for 'y' input")
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestConfirmNo(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("n\n"))
	w.Close()

	result := confirm("Proceed?")
	if result {
		t.Error("expected false for 'n' input")
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestConfirmDefaultNo(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("\n"))
	w.Close()

	result := confirm("Proceed?")
	if result {
		t.Error("expected false for empty input")
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestReadLine(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("hello world\n"))
	w.Close()

	result, err := readLine()
	if err != nil {
		t.Fatalf("readLine() returned error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestReadLineWithWhitespace(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("  spaced input  \n"))
	w.Close()

	result, err := readLine()
	if err != nil {
		t.Fatalf("readLine() returned error: %v", err)
	}
	if result != "spaced input" {
		t.Errorf("expected 'spaced input', got %q", result)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestReadPasswordReturnsTrimmed(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("secret123\n"))
	w.Close()

	result, err := readPassword()
	if err != nil {
		t.Fatalf("readPassword() returned error: %v", err)
	}
	expected := "secret123"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestConfirmYesCapitalized(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("YES\n"))
	w.Close()

	result := confirm("Proceed?")
	if !result {
		t.Error("expected true for 'YES' input")
	}

	os.Stdin = oldStdin
	resetStdinReader()
}

func TestReadLineMultiple(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	resetStdinReader()
	os.Stdin = r

	w.Write([]byte("first\nsecond\nthird\n"))
	w.Close()

	r1, err := readLine()
	if err != nil {
		t.Fatalf("readLine() returned error: %v", err)
	}
	if r1 != "first" {
		t.Errorf("expected 'first', got %q", r1)
	}

	r2, err := readLine()
	if err != nil {
		t.Fatalf("readLine() returned error: %v", err)
	}
	if r2 != "second" {
		t.Errorf("expected 'second', got %q", r2)
	}

	r3, err := readLine()
	if err != nil {
		t.Fatalf("readLine() returned error: %v", err)
	}
	if r3 != "third" {
		t.Errorf("expected 'third', got %q", r3)
	}

	os.Stdin = oldStdin
	resetStdinReader()
}
