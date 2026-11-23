// Command prototype-browse-node launches the native TypeScript SDK node in a
// fresh environment. It never starts Browse or reads upstream credentials.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := flag.String("connection", "", "server-generated browse-node-connection.json file")

	flag.Parse()

	if *path == "" {
		return errors.New("provide -connection with the server-generated browser node connection file")
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		return errors.New("cannot read browser node connection file")
	}

	var connection struct {
		MCPEndpoint  string `json:"mcpEndpoint"`
		MCPToken     string `json:"mcpToken"`
		CodexBaseURL string `json:"codexBaseUrl"`
		CodexToken   string `json:"codexToken"`
	}
	if json.Unmarshal(data, &connection) != nil || connection.MCPToken == "" || connection.CodexToken == "" {
		return errors.New("invalid browser node connection file")
	}

	mcpURL, err := url.Parse(connection.MCPEndpoint)
	if err != nil || mcpURL.Scheme != "http" || (mcpURL.Hostname() != "127.0.0.1" && mcpURL.Hostname() != "::1") || mcpURL.Path != "/mcp/browse" || mcpURL.User != nil || mcpURL.RawQuery != "" {
		return errors.New("expected a loopback Alloy Browse endpoint")
	}

	codeURL, err := url.Parse(connection.CodexBaseURL)
	if err != nil || codeURL.Scheme != "http" || codeURL.Host != mcpURL.Host || codeURL.Path != "/codex" || codeURL.User != nil || codeURL.RawQuery != "" {
		return errors.New("Codex and Browse must use the same Alloy server")
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	script := filepath.Join(root, "cmd/browsenode/dist/run.js")
	if _, err := os.Stat(script); err != nil {
		return errors.New("build the node first: pnpm --dir cmd/browsenode build")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		return err
	}

	workspace, err := os.MkdirTemp("", "alloy-browse-node-")
	if err != nil {
		return err
	}

	home := filepath.Join(workspace, "home")

	codeHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codeHome, 0700); err != nil {
		return err
	}

	log.Printf("Fresh browser-node workspace: %s (environment separation, not a sandbox)", workspace)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, node, script)
	cmd.Dir = workspace

	cmd.Env = []string{
		"HOME=" + home, "CODEX_HOME=" + codeHome, "TMPDIR=" + workspace, "PATH=/usr/bin:/bin",
		"ALLOY_BROWSE_URL=" + connection.MCPEndpoint, "ALLOY_BROWSE_TOKEN=" + connection.MCPToken,
		"ALLOY_CODEX_URL=" + connection.CodexBaseURL, "ALLOY_CODEX_TOKEN=" + connection.CodexToken,
	}
	if model := os.Getenv("ALLOY_CODEX_MODEL"); model != "" {
		cmd.Env = append(cmd.Env, "ALLOY_CODEX_MODEL="+model)
	}

	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("browser node failed: %w", err)
	}

	return nil
}
