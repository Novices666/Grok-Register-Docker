package cpa

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendGrok2APITokenWritesRawTokenOnly(t *testing.T) {
	dir := t.TempDir()

	if err := AppendGrok2APIToken(dir, "sso-token-one"); err != nil {
		t.Fatal(err)
	}
	if err := AppendGrok2APIToken(dir, "sso-token-two"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "tokens.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), "sso-token-one\nsso-token-two\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}
