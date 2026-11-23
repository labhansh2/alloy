package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"alloy/internal/prototype/mcpgateway"
	"alloy/internal/prototype/mcpoauth"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func registerNotion(ctx context.Context, mux *http.ServeMux, baseURL, stateDir string, reauthorize bool) (func(), error) {
	flow := mcpoauth.NewLoopback()
	mux.Handle("GET "+flow.StartPath, flow)
	mux.Handle("GET /oauth/callback", flow)

	providerHTTP, err := mcpoauth.OriginClient("https://mcp.notion.com")
	if err != nil {
		return nil, err
	}

	providerHTTP.Timeout = 60 * time.Second
	store := mcpoauth.Store{Path: filepath.Join(stateDir, "notion-oauth.json")}

	initial, err := store.Load(ctx, providerHTTP)
	if err != nil && !reauthorize {
		return nil, err
	}

	if reauthorize {
		initial = nil
	}

	oauthHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL: baseURL + "/oauth/callback",
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			ClientName: "Alloy Notion prototype", RedirectURIs: []string{baseURL + "/oauth/callback"},
			GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none",
		}},
		RequestRefreshToken: true, Client: providerHTTP, InitialTokenSource: initial, NewTokenSource: store.NewTokenSource,
		AuthorizationCodeFetcher: func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			log.Printf("Notion consent required. Open %s%s", baseURL, flow.StartPath)

			loginCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			return flow.Fetch(loginCtx, args)
		},
	})
	if err != nil {
		return nil, errors.New("cannot configure Notion OAuth client")
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "alloy-notion-connection", Version: "prototype"}, nil)

	upstream, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: "https://mcp.notion.com/mcp", HTTPClient: providerHTTP, OAuthHandler: oauthHandler, DisableStandaloneSSE: true, MaxRetries: -1, MaxEventSize: 1 << 20,
	}, nil)
	if err != nil {
		return nil, errors.New("Notion MCP connection failed; complete consent or reconnect the server (upstream details suppressed)")
	}

	ok := false

	defer func() {
		if !ok {
			_ = upstream.Close()
		}
	}()

	token := rand.Text()

	handler, err := mcpgateway.NewTools(ctx, upstream, mcpgateway.Options{Token: token, AllowedTools: []string{"notion-search"}, CallTimeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}

	mux.Handle("/mcp/notion", handler)

	path := filepath.Join(stateDir, "notion-node-connection.json")
	if err := mcpoauth.SavePrivateJSON(path, struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}{baseURL + "/mcp/notion", token}); err != nil {
		return nil, err
	}

	log.Printf("Notion ready: %s/mcp/notion; node connection: %s", baseURL, path)

	ok = true

	return func() { _ = upstream.Close() }, nil
}
