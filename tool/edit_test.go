package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditSingleReplacement(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "e.txt", "hello world\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"e.txt","old_string":"hello","new_string":"goodbye"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "e.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "goodbye world\n" {
		t.Errorf("expected 'goodbye world\\n', got %q", string(data))
	}
}

func TestEditReplaceAll(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "ra.txt", "aaa bbb aaa\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"ra.txt","old_string":"aaa","new_string":"ccc","replace_all":true}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ra.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ccc bbb ccc\n" {
		t.Errorf("expected 'ccc bbb ccc\\n', got %q", string(data))
	}
}

func TestEditMultipleWithoutReplaceAllFails(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "m.txt", "foo bar foo\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"m.txt","old_string":"foo","new_string":"baz"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string matches multiple times without replace_all")
	}
}

func TestEditNotFound(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "nf.txt", "hello\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"nf.txt","old_string":"missing","new_string":"x"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string not found")
	}
}

func TestEditIdenticalStrings(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "id.txt", "same\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"id.txt","old_string":"same","new_string":"same"}`)
	if r.Error == nil {
		t.Fatal("expected error when old_string and new_string are identical")
	}
}

func TestEditMissingFile(t *testing.T) {
	ctx, _ := makeTestCtx(t)
	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"nope.txt","old_string":"a","new_string":"b"}`)
	if r.Error == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEditMultilineReplacement(t *testing.T) {
	ctx, dir := makeTestCtx(t)
	writeTestFile(t, dir, "ml.txt", "line1\nline2\nline3\n")

	tool := NewEdit()
	r := tool.Execute(ctx, `{"file_path":"ml.txt","old_string":"line1\nline2","new_string":"replaced"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ml.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "replaced\nline3\n" {
		t.Errorf("expected 'replaced\\nline3\\n', got %q", string(data))
	}
}
