// Package nodesession implements the initial, local-only node handshake.
// This is an internal protocol, not a stable resource or event API.
package nodesession

import "fmt"

const (
	Path     = "/alloy/node"
	Version  = 1
	MaxBytes = 4096
)

type Message struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Name    string `json:"name,omitempty"`
	Client  string `json:"client,omitempty"`
}

// ValidateName checks descriptive metadata, not an authenticated identity.
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 128 {
		return fmt.Errorf("node name must contain 1–128 ASCII letters, digits, dots, underscores, or hyphens")
	}

	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}

		return fmt.Errorf("invalid character in node name")
	}

	return nil
}
