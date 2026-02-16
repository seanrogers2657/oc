package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCreatesFile(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	tool := NewWrite()
	r := tool.Execute(ctx, `{"file_path":"new.txt","content":"hello world"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestWriteCreatesParentDirs(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	tool := NewWrite()
	r := tool.Execute(ctx, `{"file_path":"a/b/c/deep.txt","content":"deep"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "a/b/c/deep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "deep" {
		t.Errorf("expected 'deep', got %q", string(data))
	}
}

func TestWriteOverwritesExisting(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "exist.txt", "old content")

	tool := NewWrite()
	r := tool.Execute(ctx, `{"file_path":"exist.txt","content":"new content"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "exist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Errorf("expected 'new content', got %q", string(data))
	}
}

func TestWriteEmptyFilePath(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewWrite()
	r := tool.Execute(ctx, `{"file_path":"","content":"x"}`)
	if r.Error == nil {
		t.Fatal("expected error for empty file_path")
	}
}

func TestWriteTitle(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewWrite()
	r := tool.Execute(ctx, `{"file_path":"out.txt","content":"data"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Title, "out.txt") {
		t.Errorf("title should contain filename, got: %s", r.Title)
	}
	if !strings.Contains(r.Title, "4 bytes") {
		t.Errorf("title should contain byte count, got: %s", r.Title)
	}
}
