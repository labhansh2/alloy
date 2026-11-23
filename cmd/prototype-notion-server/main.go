// Command prototype-notion-server configures the application-specific Notion
// connection and read-only node grant. Generic MCP/OAuth plumbing is internal.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"alloy/internal/prototype/mcpgateway"
	"alloy/internal/prototype/mcpoauth"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	stateDir := flag.String("state-dir", filepath.Join(configDir, "alloy", "prototype-notion"), "private server OAuth/cache directory outside the repo")
	reauthorize := flag.Bool("reauthorize", false, "request fresh browser consent instead of reusing the OAuth cache")
	flag.Parse()
	if err := os.MkdirAll(*stateDir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(*stateDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("state directory must be private (0700)")
	}
	// Prototype target: macOS/Linux. Prevent concurrent refresh-token writers.
	lock, err := os.OpenFile(filepath.Join(*stateDir, "server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another server owns this OAuth directory")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	baseURL := "http://" + listener.Addr().String()
	flow := mcpoauth.NewLoopback()
	mux := http.NewServeMux()
	mux.Handle("GET "+flow.StartPath, flow)
	mux.Handle("GET /oauth/callback", flow)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() { _ = server.Close(); <-done }()

	providerHTTP, err := mcpoauth.OriginClient("https://mcp.notion.com")
	if err != nil {
		return err
	}
	providerHTTP.Timeout = 60 * time.Second
	store := mcpoauth.Store{Path: filepath.Join(*stateDir, "notion-oauth.json")}
	initial, err := store.Load(ctx, providerHTTP)
	if err != nil && !*reauthorize {
		return err
	}
	if *reauthorize {
		initial = nil
	}
	oauthHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL: baseURL + "/oauth/callback",
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			ClientName: "Alloy Notion prototype", RedirectURIs: []string{baseURL + "/oauth/callback"},
			GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none",
		}},
		RequestRefreshToken: true,
		Client:              providerHTTP, InitialTokenSource: initial, NewTokenSource: store.NewTokenSource,
		AuthorizationCodeFetcher: func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			log.Printf("Notion consent required. Open %s%s", baseURL, flow.StartPath)
			loginCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			return flow.Fetch(loginCtx, args)
		},
	})
	if err != nil {
		return errors.New("cannot configure Notion OAuth client")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "alloy-notion-connection", Version: "prototype"}, nil)
	upstream, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: "https://mcp.notion.com/mcp", HTTPClient: providerHTTP, OAuthHandler: oauthHandler, DisableStandaloneSSE: true, MaxRetries: -1,
		MaxEventSize: 1 << 20,
	}, nil)
	if err != nil {
		return errors.New("Notion MCP connection failed; complete consent or reconnect the server (upstream details suppressed)")
	}
	defer upstream.Close()
	nodeToken := rand.Text()
	handler, err := mcpgateway.NewTools(ctx, upstream, mcpgateway.Options{Token: nodeToken, AllowedTools: []string{"notion-search"}, CallTimeout: 30 * time.Second})
	if err != nil {
		return err
	}
	mux.Handle("/mcp/notion", handler)
	connectionPath := filepath.Join(*stateDir, "node-connection.json")
	if err := mcpoauth.SavePrivateJSON(connectionPath, struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}{Endpoint: baseURL + "/mcp/notion", Token: nodeToken}); err != nil {
		return err
	}
	log.Printf("Ready: %s/mcp/notion (only notion-search)", baseURL)
	log.Printf("Node connection file (no Notion tokens): %s", connectionPath)
	log.Println("OAuth is stored server-side; node credential is rotated on every server start. Ctrl+C to stop.")
	select {
	case <-ctx.Done():
		return nil
	case err := <-done:
		// Return the result to the deferred join without losing it.
		done <- err
		return fmt.Errorf("HTTP server stopped: %w", err)
	}
}
