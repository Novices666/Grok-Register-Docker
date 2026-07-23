package webui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTailFileReturnsLastLinesFromLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.log")
	var b strings.Builder
	for i := 1; i <= 500; i++ {
		b.WriteString("line-")
		b.WriteString(strings.Repeat("x", 8))
		b.WriteByte('-')
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := tailFile(path, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 20 {
		t.Fatalf("len(lines)=%d, want 20", len(lines))
	}
	if !strings.HasSuffix(lines[0], "-481") {
		t.Fatalf("first tailed line = %q, want suffix -481", lines[0])
	}
	if !strings.HasSuffix(lines[len(lines)-1], "-500") {
		t.Fatalf("last tailed line = %q, want suffix -500", lines[len(lines)-1])
	}
}

func TestTailFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := tailFile(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("len(lines)=%d, want 0", len(lines))
	}
}
