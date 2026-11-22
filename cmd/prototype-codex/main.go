// Command prototype-codex runs a clean-environment node SDK against Alloy's
// host-auth proxy. This is environment separation, not a security sandbox.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"alloy/internal/prototype/codexproxy"
	"alloy/internal/prototype/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	script := filepath.Join(root, "cmd/codexnode/dist/run.js")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("build the TypeScript node first: pnpm --dir cmd/codexnode build: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	authHome := os.Getenv("CODEX_HOME")
	if authHome == "" {
		authHome = filepath.Join(home, ".codex")
	}
	token := rand.Text()
	proxy, err := codexproxy.New(filepath.Join(authHome, "auth.json"), token)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/codex/", proxy)
	mux.Handle("/", server.NewHandler(log.Default()))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = httpServer.Serve(listener)
	}()
	defer func() {
		_ = httpServer.Close()
		<-done
	}()

	workspace, err := os.MkdirTemp("", "alloy-codex-node-")
	if err != nil {
		return err
	}
	// Deliberately retain the workspace for inspection; it contains no host auth.
	log.Printf("clean node workspace: %s", workspace)
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
		"HOME=" + nodeHome, "CODEX_HOME=" + codeHome,
		"PATH=/usr/bin:/bin", "TMPDIR=" + workspace,
		"ALLOY_CODEX_URL=http://" + listener.Addr().String() + "/codex",
		"ALLOY_NODE_TOKEN=" + token,
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
