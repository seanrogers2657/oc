package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestCtx(t *testing.T) (Context, string) {
	t.Helper()
	dir := t.TempDir()
	return Context{WorkingDir: dir, Ctx: context.Background()}, dir
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadBasic(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "hello.txt", "line1\nline2\nline3\n")

	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":"hello.txt"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "line1") {
		t.Error("expected line1 in output")
	}
	if !strings.Contains(r.Output, "line3") {
		t.Error("expected line3 in output")
	}
	// Line numbers should be present
	if !strings.Contains(r.Output, "1\t") {
		t.Error("expected line numbers")
	}
}

func TestReadWithOffset(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "lines.txt", "a\nb\nc\nd\ne\n")

	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":"lines.txt","offset":3,"limit":2}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "c") {
		t.Error("expected line 'c' at offset 3")
	}
	if !strings.Contains(r.Output, "d") {
		t.Error("expected line 'd'")
	}
	if strings.Contains(r.Output, "\ta\n") {
		t.Error("should not contain line 'a'")
	}
}

func TestReadAbsolutePath(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	absPath := writeTestFile(t, dir, "abs.txt", "absolute\n")

	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":"`+absPath+`"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "absolute") {
		t.Error("expected content")
	}
}

func TestReadMissingFile(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":"nope.txt"}`)
	if r.Error == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadEmptyFilePath(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":""}`)
	if r.Error == nil {
		t.Fatal("expected error for empty file_path")
	}
}

func TestReadTitle(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "title.txt", "one\ntwo\n")

	tool := NewRead()
	r := tool.Execute(ctx, `{"file_path":"title.txt"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Title, "title.txt") {
		t.Errorf("title should contain filename, got: %s", r.Title)
	}
}
