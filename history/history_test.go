package history

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStoreAt(filepath.Join(dir, "history.json"))
}

func TestLoadMissingFile(t *testing.T) {
	s := tempStore(t)
	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Load() = %v, want empty", entries)
	}
}

func TestAppendAndLoad(t *testing.T) {
	s := tempStore(t)

	if err := s.Append("hello"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := s.Append("world"); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 || entries[0] != "hello" || entries[1] != "world" {
		t.Fatalf("Load() = %v, want [hello world]", entries)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	s := tempStore(t)
	os.WriteFile(s.path, []byte("not json{{{"), 0600)

	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error on corrupt file: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Load() = %v, want empty for corrupt file", entries)
	}
}

func TestAppendMaxCap(t *testing.T) {
	s := tempStore(t)

	// Write maxEntries + 10 entries
	for i := 0; i < maxEntries+10; i++ {
		if err := s.Append("entry"); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	entries, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != maxEntries {
		t.Fatalf("len = %d, want %d", len(entries), maxEntries)
	}
}

func TestFilePermissions(t *testing.T) {
	s := tempStore(t)
	if err := s.Append("test"); err != nil {
		t.Fatalf("Append: %v", err)
	}

	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("file perm = %o, want 0600", perm)
	}
}
