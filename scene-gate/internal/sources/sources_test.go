package sources

import (
	os
	"path/filepath"
	testing
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sources.yaml")
	content := "version: 1\nsources:\n  - id: front-cam-01\n    description: test\n  - id: \"\"\n  - id: cabin-1\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2, got %d", len(entries))
	}
	if entries[0].ID != "front-cam-01" || entries[1].ID != "cabin-1" {
		t.Fatalf("got %+v", entries)
	}
}

func TestLoadMissing(t *testing.T) {
	entries, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("want empty, got %v", entries)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	entries, err := Load("")
	if err != nil || len(entries) != 0 {
		t.Fatalf("err=%v entries=%v", err, entries)
	}
}
