package tool

import (
	"strings"
	"testing"
)

func TestGlobMatchesGoFiles(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "main.go", "package main")
	writeTestFile(t, dir, "util.go", "package util")
	writeTestFile(t, dir, "readme.md", "# readme")

	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":"*.go"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "main.go") {
		t.Error("expected main.go in output")
	}
	if !strings.Contains(r.Output, "util.go") {
		t.Error("expected util.go in output")
	}
	if strings.Contains(r.Output, "readme.md") {
		t.Error("should not contain readme.md")
	}
}

func TestGlobDoublestar(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "a/b/c.go", "pkg")
	writeTestFile(t, dir, "d.go", "pkg")
	writeTestFile(t, dir, "a/e.txt", "txt")

	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":"**/*.go"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "c.go") {
		t.Error("expected a/b/c.go in output")
	}
	if !strings.Contains(r.Output, "d.go") {
		t.Error("expected d.go in output")
	}
	if strings.Contains(r.Output, "e.txt") {
		t.Error("should not contain e.txt")
	}
}

func TestGlobWithPath(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "src/app.go", "pkg")
	writeTestFile(t, dir, "test/app_test.go", "pkg")

	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":"*.go","path":"src"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "app.go") {
		t.Error("expected app.go")
	}
	if strings.Contains(r.Output, "app_test.go") {
		t.Error("should not contain files outside src/")
	}
}

func TestGlobNoMatches(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":"*.xyz"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "No files found") {
		t.Error("expected 'No files found'")
	}
}

func TestGlobSkipsGitDir(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, ".git/config", "gitconfig")
	writeTestFile(t, dir, "real.go", "pkg")

	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":"**/*"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if strings.Contains(r.Output, ".git") {
		t.Error("should skip .git directory")
	}
	if !strings.Contains(r.Output, "real.go") {
		t.Error("expected real.go")
	}
}

func TestGlobEmptyPattern(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewGlob()
	r := tool.Execute(ctx, `{"pattern":""}`)
	if r.Error == nil {
		t.Fatal("expected error for empty pattern")
	}
}
