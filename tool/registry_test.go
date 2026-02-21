package tool

import "testing"

// dummyTool is a minimal Tool for testing the registry.
type dummyTool struct {
	name string
}

func (d *dummyTool) Name() string                   { return d.name }
func (d *dummyTool) Description() string            { return d.name + " description" }
func (d *dummyTool) Parameters() map[string]any     { return map[string]any{"type": "object"} }
func (d *dummyTool) Execute(Context, string) Result { return Result{Output: d.name} }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&dummyTool{name: "alpha"})
	r.Register(&dummyTool{name: "beta"})

	tool, ok := r.Get("alpha")
	if !ok {
		t.Fatal("expected to find alpha")
	}
	if tool.Name() != "alpha" {
		t.Fatalf("expected alpha, got %s", tool.Name())
	}

	_, ok = r.Get("missing")
	if ok {
		t.Fatal("expected not found for missing tool")
	}
}

func TestRegistryAllPreservesOrder(t *testing.T) {
	r := NewToolRegistry()
	names := []string{"charlie", "alpha", "bravo"}
	for _, n := range names {
		r.Register(&dummyTool{name: n})
	}

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(all))
	}
	for i, n := range names {
		if all[i].Name() != n {
			t.Errorf("position %d: expected %s, got %s", i, n, all[i].Name())
		}
	}
}

func TestRegistryReRegisterKeepsOrder(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&dummyTool{name: "a"})
	r.Register(&dummyTool{name: "b"})
	// Re-register "a" with a new instance
	r.Register(&dummyTool{name: "a"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools after re-register, got %d", len(all))
	}
	if all[0].Name() != "a" || all[1].Name() != "b" {
		t.Errorf("unexpected order: %s, %s", all[0].Name(), all[1].Name())
	}
}

func TestRegistryDefs(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&dummyTool{name: "tool1"})
	r.Register(&dummyTool{name: "tool2"})

	defs := r.Defs()
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if defs[0].Name != "tool1" {
		t.Errorf("expected tool1, got %s", defs[0].Name)
	}
	if defs[1].Description != "tool2 description" {
		t.Errorf("unexpected description: %s", defs[1].Description)
	}
}

func TestRegistryEmpty(t *testing.T) {
	r := NewToolRegistry()
	if len(r.All()) != 0 {
		t.Fatal("expected empty registry")
	}
	if len(r.Defs()) != 0 {
		t.Fatal("expected no defs")
	}
	_, ok := r.Get("anything")
	if ok {
		t.Fatal("expected not found")
	}
}
