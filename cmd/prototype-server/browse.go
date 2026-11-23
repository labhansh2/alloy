package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alloy/internal/prototype/mcpgateway"
	"alloy/internal/prototype/mcpoauth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Provider choices, browser session, origins and node grants are application
// configuration, not engine defaults. Browse itself stays on the server.
var browseOrigins = []string{"https://example.com", "https://www.iana.org"}

func registerBrowse(ctx context.Context, mux *http.ServeMux, baseURL, stateDir, codexToken string) (func(), error) {
	path, err := exec.LookPath("browse")
	if err != nil {
		return nil, errors.New("browse is not installed on the server")
	}

	cmd := exec.Command(path, "mcp")

	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=/usr/bin:/bin:/opt/homebrew/bin"}
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		if value := os.Getenv(name); value != "" {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "alloy-browse-connection", Version: "prototype"}, nil)

	upstream, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}, nil)
	if err != nil {
		return nil, errors.New("cannot start Browse MCP on the server")
	}

	sessionID := "alloy-" + strings.ToLower(rand.Text())
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, _ = upstream.CallTool(cleanupCtx, &mcp.CallToolParams{Name: "session_delete", Arguments: map[string]any{"id": sessionID}})
		_ = upstream.Close()
	}
	ok := false

	defer func() {
		if !ok {
			cleanup()
		}
	}()

	setupCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	result, err := upstream.CallTool(setupCtx, &mcp.CallToolParams{Name: "session_new", Arguments: map[string]any{
		"id": sessionID, "allowedOrigins": browseOrigins,
	}})
	if err != nil || result.IsError {
		return nil, errors.New("Browse session setup failed; connect the paired browser extension and retry")
	}

	browseToken := rand.Text()

	handler, err := mcpgateway.NewTools(setupCtx, upstream, mcpgateway.Options{
		Token: browseToken, AllowedTools: []string{"execute"}, CallTimeout: 60 * time.Second, FilterCall: pinBrowseSession(sessionID),
	})
	if err != nil {
		return nil, err
	}

	mux.Handle("/mcp/browse", handler)

	connectionPath := filepath.Join(stateDir, "browse-node-connection.json")

	connection := struct {
		MCPEndpoint  string `json:"mcpEndpoint"`
		MCPToken     string `json:"mcpToken"`
		CodexBaseURL string `json:"codexBaseUrl"`
		CodexToken   string `json:"codexToken"`
	}{baseURL + "/mcp/browse", browseToken, baseURL + "/codex", codexToken}
	if err := mcpoauth.SavePrivateJSON(connectionPath, connection); err != nil {
		return nil, err
	}

	log.Printf("Browse ready: %s/mcp/browse (execute only; dedicated public-site session)", baseURL)
	log.Printf("Browser node connection file (no upstream credentials): %s", connectionPath)

	ok = true

	return cleanup, nil
}

// Nodes cannot adopt another session/tab or pick another browser. The tool's
// original schema remains native Browse; the server injects its own session.
func pinBrowseSession(sessionID string) func(string, json.RawMessage) (json.RawMessage, error) {
	return func(name string, raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if name != "execute" || json.Unmarshal(raw, &args) != nil || args == nil {
			return nil, errors.New("invalid Browse call")
		}

		for _, field := range []string{"browser", "targetUrl", "targetIndex"} {
			if _, ok := args[field]; ok {
				return nil, errors.New("Browse target overrides are not permitted")
			}
		}

		if requested, ok := args["session"]; ok && requested != sessionID {
			return nil, errors.New("Browse session override is not permitted")
		}

		if code, ok := args["code"].(string); !ok || strings.TrimSpace(code) == "" {
			return nil, errors.New("Browse code is required")
		}

		args["session"] = sessionID
		args["timeoutMs"] = 45000

		return json.Marshal(args)
	}
}
