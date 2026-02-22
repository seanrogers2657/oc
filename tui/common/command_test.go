package common

import "testing"

func TestNewCommandRegistry(t *testing.T) {
	r := NewCommandRegistry()
	if r == nil {
		t.Fatal("NewCommandRegistry returned nil")
	}
	if len(r.All()) != 0 {
		t.Fatalf("new registry should be empty, got %d commands", len(r.All()))
	}
}

func TestCommandRegistryRegister(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "test", Description: "a test command"})

	cmds := r.All()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", cmds[0].Name)
	}
	if cmds[0].Description != "a test command" {
		t.Errorf("expected description 'a test command', got %q", cmds[0].Description)
	}
}

func TestCommandRegistryMultiple(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "a"})
	r.Register(Command{Name: "b"})
	r.Register(Command{Name: "c"})

	cmds := r.All()
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	// Order preserved
	if cmds[0].Name != "a" || cmds[1].Name != "b" || cmds[2].Name != "c" {
		t.Errorf("commands not in registration order: %v", cmds)
	}
}

func TestCommandWithAliases(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{
		Name:    "quit",
		Aliases: []string{"q", "exit"},
	})

	cmd := r.All()[0]
	if len(cmd.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(cmd.Aliases))
	}
	if cmd.Aliases[0] != "q" || cmd.Aliases[1] != "exit" {
		t.Errorf("aliases = %v, want [q, exit]", cmd.Aliases)
	}
}

func TestCommandAction(t *testing.T) {
	called := false
	r := NewCommandRegistry()
	r.Register(Command{
		Name:   "test",
		Action: func() { called = true },
	})

	r.All()[0].Action()
	if !called {
		t.Error("action was not called")
	}
}
