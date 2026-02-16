package tool

import (
	"strings"
	"testing"
)

func TestGrepBasicMatch(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "hello.go", "package main\n\nfunc hello() {}\nfunc world() {}\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"hello"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "hello") {
		t.Error("expected match on 'hello'")
	}
	if !strings.Contains(r.Output, "hello.go") {
		t.Error("expected filename in output")
	}
}

func TestGrepRegex(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "data.txt", "foo123\nbar456\nbaz789\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"[a-z]+\\d{3}"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	// All 3 lines should match
	if !strings.Contains(r.Title, "3 matches") {
		t.Errorf("expected 3 matches, got title: %s", r.Title)
	}
}

func TestGrepWithGlobFilter(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "code.go", "func main() {}\n")
	writeTestFile(t, dir, "notes.txt", "func notes\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"func","glob":"*.go"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "code.go") {
		t.Error("expected code.go in results")
	}
	if strings.Contains(r.Output, "notes.txt") {
		t.Error("should not contain notes.txt when filtering by *.go")
	}
}

func TestGrepWithPath(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "src/a.go", "target\n")
	writeTestFile(t, dir, "lib/b.go", "target\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"target","path":"src"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "a.go") {
		t.Error("expected a.go")
	}
	if strings.Contains(r.Output, "b.go") {
		t.Error("should not search in lib/")
	}
}

func TestGrepWithContextLines(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "ctx.txt", "line1\nline2\nMATCH\nline4\nline5\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"MATCH","context":1}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "line2") {
		t.Error("expected context line before match")
	}
	if !strings.Contains(r.Output, "line4") {
		t.Error("expected context line after match")
	}
}

func TestGrepNoMatches(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "empty.txt", "nothing here\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"zzzzz"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "No matches") {
		t.Error("expected 'No matches found'")
	}
}

func TestGrepInvalidRegex(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"[invalid"}`)
	if r.Error == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestGrepEmptyPattern(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":""}`)
	if r.Error == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestGrepSkipsBinaryFiles(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "image.png", "target inside binary\n")
	writeTestFile(t, dir, "text.txt", "target inside text\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"target"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if strings.Contains(r.Output, "image.png") {
		t.Error("should skip .png files")
	}
	if !strings.Contains(r.Output, "text.txt") {
		t.Error("expected text.txt in results")
	}
}

func TestGrepSingleFile(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "single.txt", "alpha\nbeta\ngamma\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"beta","path":"single.txt"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !strings.Contains(r.Output, "beta") {
		t.Error("expected beta match")
	}
	if !strings.Contains(r.Title, "1 match") {
		t.Errorf("expected 1 match, got: %s", r.Title)
	}
}

func TestGrepSkipsGitDir(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeTestFile(t, dir, "code.txt", "ref: something\n")

	tool := NewGrep()
	r := tool.Execute(ctx, `{"pattern":"ref:"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if strings.Contains(r.Output, ".git") {
		t.Error("should not search in .git/")
	}
	if !strings.Contains(r.Output, "code.txt") {
		t.Error("expected code.txt match")
	}
}
