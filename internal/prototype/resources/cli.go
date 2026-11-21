package resources

import (
	"context"
	"encoding/json"
	"os/exec"

	"alloy/internal/prototype/nodewire"
)

// PrototypeCLI returns narrowly scoped server-side CLI resources. It does not
// accept executable names or shell fragments from a node.
func PrototypeCLI() map[string]nodewire.ResourceHandler {
	return map[string]nodewire.ResourceHandler{
		"echo": echo,
		"ls":   listWorkingDirectory,
	}
}

func echo(ctx context.Context, input json.RawMessage) (any, error) {
	var request struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, err
	}
	output, err := exec.CommandContext(ctx, "echo", request.Text).Output()
	if err != nil {
		return nil, err
	}
	return map[string]string{"output": string(output)}, nil
}

func listWorkingDirectory(ctx context.Context, _ json.RawMessage) (any, error) {
	output, err := exec.CommandContext(ctx, "ls", "-la").Output()
	if err != nil {
		return nil, err
	}
	return map[string]string{"output": string(output)}, nil
}
