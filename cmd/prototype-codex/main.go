// Command prototype-codex launches a clean-environment SDK node connected to
// the shared prototype server. It does not start a server or read host auth.
package main

import (
	"context"
	"crypto/rand"
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
	path := flag.String("connection", "", "server-generated codex-node-connection.json file")

	flag.Parse()

	if *path == "" {
		return errors.New("provide -connection with the shared server's Codex node connection file")
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		return errors.New("cannot read Codex node connection file")
	}

	var connection struct {
		BaseURL string `json:"baseUrl"`
		Token   string `json:"token"`
	}
	if json.Unmarshal(data, &connection) != nil || connection.Token == "" {
		return errors.New("invalid Codex node connection file")
	}

	u, err := url.Parse(connection.BaseURL)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Path != "/codex" || u.User != nil || u.RawQuery != "" {
		return errors.New("expected a loopback Alloy Codex endpoint")
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	script := filepath.Join(root, "cmd/codexnode/dist/run.js")
	if _, err := os.Stat(script); err != nil {
		return errors.New("build the node first: pnpm --dir cmd/codexnode build")
	}

	workspace, err := os.MkdirTemp("", "alloy-codex-node-")
	if err != nil {
		return err
	}

	log.Printf("clean node workspace: %s (environment separation, not a sandbox)", workspace)
	nodeHome := filepath.Join(workspace, "home")

	codeHome := filepath.Join(nodeHome, ".codex")
	if err := os.MkdirAll(codeHome, 0700); err != nil {
		return err
	}

	marker := "ALLOY_NODE_" + rand.Text()
	if err := os.WriteFile(filepath.Join(workspace, "node-marker.txt"), []byte(marker+"\n"), 0600); err != nil {
		return err
	}

	node, err := exec.LookPath("node")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, node, script)
	cmd.Dir = workspace

	cmd.Env = []string{
		"HOME=" + nodeHome, "CODEX_HOME=" + codeHome, "PATH=/usr/bin:/bin", "TMPDIR=" + workspace,
		"ALLOY_CODEX_URL=" + connection.BaseURL, "ALLOY_NODE_TOKEN=" + connection.Token,
	}
	if model := os.Getenv("ALLOY_CODEX_MODEL"); model != "" {
		cmd.Env = append(cmd.Env, "ALLOY_CODEX_MODEL="+model)
	}

	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("node Codex proof failed: %w", err)
	}

	return nil
}
