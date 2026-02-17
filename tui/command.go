package tui

// Command represents an executable action in the command palette.
type Command struct {
	Name        string
	Description string
	Action      func()
	Aliases     []string
}

// CommandRegistry holds a list of available commands.
type CommandRegistry struct {
	commands []Command
}

// NewCommandRegistry creates an empty command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{}
}

// Register adds a command to the registry.
func (r *CommandRegistry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
}

// All returns all registered commands.
func (r *CommandRegistry) All() []Command {
	return r.commands
}
