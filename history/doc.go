// Package history persists prompt history to disk so previously submitted
// messages survive across sessions.
//
// Store reads and writes a JSON file at ~/.oc/history.json, capping entries
// at a fixed maximum to bound disk usage.
//
// This package has no dependencies beyond stdlib.
package history
