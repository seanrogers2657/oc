package history

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const maxEntries = 500

// Store persists prompt history to ~/.oc/history.json.
type Store struct {
	path string
}

// NewStore returns a store using the default path ~/.oc/history.json.
func NewStore() *Store {
	home, _ := os.UserHomeDir()
	return &Store{path: filepath.Join(home, ".oc", "history.json")}
}

// NewStoreAt returns a store using a custom path (for testing).
func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

// Load reads the stored history. Returns an empty slice if the file doesn't exist.
func (s *Store) Load() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		// Corrupt file — start fresh
		return nil, nil
	}
	return entries, nil
}

// Append adds a new entry to the history file, trimming to maxEntries.
func (s *Store) Append(entry string) error {
	entries, _ := s.Load()
	entries = append(entries, entry)
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	return s.write(entries)
}

func (s *Store) write(entries []string) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
