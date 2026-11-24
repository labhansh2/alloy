package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"

	"alloy/internal/prototype/mcpoauth"
	"alloy/internal/prototype/nodewire"
)

func registerCLI(mux *http.ServeMux, baseURL, stateDir string) error {
	token := rand.Text()
	protocol := &nodewire.Server{NodeID: "prototype-node", Resources: map[string]nodewire.ResourceHandler{
		"echo": echo,
		"ls":   listWorkingDirectory,
	}, Authorize: func(node, resource string) bool {
		return node == "prototype-node" && (resource == "echo" || resource == "ls")
	},
		OnPublish: func(_ context.Context, node string, payload json.RawMessage) error {
			log.Printf("event from node %q: %s", node, payload)
			return nil
		},
	}
	mux.Handle("GET /node", nodewire.HTTPHandler(protocol, token, log.Default()))

	path := filepath.Join(stateDir, "cli-node-connection.json")
	if err := mcpoauth.SavePrivateJSON(path, struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}{"ws" + baseURL[len("http"):] + "/node", token}); err != nil {
		return err
	}

	log.Printf("CLI/event connection: %s", path)

	return nil
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
