package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMarkdownBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := "# Title\n\nThis is **markdown**."
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp markdown: %v", err)
	}

	blocks, err := loadMarkdownBlocks(path)
	if err != nil {
		t.Fatalf("loadMarkdownBlocks returned error: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatalf("expected at least one block")
	}
}
