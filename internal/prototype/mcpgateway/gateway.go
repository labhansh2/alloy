// Package mcpgateway exposes explicitly selected upstream MCP tools using the
// standard MCP transport. Application connections and policy live in cmd/.
package mcpgateway

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	// Token is a prototype, single-node credential, never an upstream token.
	Token        string
	AllowedTools []string
	CallTimeout  time.Duration
	// FilterCall applies server-owned argument constraints before forwarding.
	// It cannot expose tools outside AllowedTools. Nil preserves arguments.
	FilterCall func(name string, arguments json.RawMessage) (json.RawMessage, error)
}

// NewTools exposes only the named tools. It terminates MCP on both sides:
// upstream sessions and OAuth headers are never forwarded to downstream nodes.
// Prompts, resources, sampling, elicitation and dynamic tool changes are not
// bridged by this deliberately narrow prototype.
func NewTools(ctx context.Context, upstream *mcp.ClientSession, opts Options) (http.Handler, error) {
	if upstream == nil || opts.Token == "" || len(opts.AllowedTools) == 0 {
		return nil, errors.New("upstream, node token and explicit allowed tools are required")
	}

	if opts.CallTimeout <= 0 {
		opts.CallTimeout = 30 * time.Second
	}

	allowed := make(map[string]bool)
	for _, name := range opts.AllowedTools {
		allowed[name] = false
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "alloy-tool-gateway", Version: "prototype"}, nil)
	// Bound work across stateless downstream sessions as well as within one node.
	concurrentCalls := make(chan struct{}, 4)

	for tool, err := range upstream.Tools(ctx, nil) {
		if err != nil {
			return nil, errors.New("cannot discover upstream MCP tools")
		}

		if _, ok := allowed[tool.Name]; !ok {
			continue
		}

		allowed[tool.Name] = true
		// Use the real upstream schema; do not invent an equivalent tool API.
		server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments := req.Params.Arguments

			if opts.FilterCall != nil {
				var err error

				arguments, err = opts.FilterCall(tool.Name, arguments)
				if err != nil {
					return toolError("MCP call rejected by server policy."), nil
				}
			}

			select {
			case concurrentCalls <- struct{}{}:
				defer func() { <-concurrentCalls }()
			default:
				return toolError("MCP gateway is busy; try again later."), nil
			}

			ctx, cancel := context.WithTimeout(ctx, opts.CallTimeout)
			defer cancel()

			result, err := upstream.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: arguments})
			if err != nil {
				// Upstream errors can contain sensitive HTTP/OAuth details.
				return toolError("Upstream MCP call failed; check the server connection."), nil
			}

			encoded, err := json.Marshal(result)
			if err != nil || len(encoded) > 1<<20 {
				return toolError("Upstream result exceeds the prototype response limit."), nil
			}

			return result, nil
		})
	}

	for name, found := range allowed {
		if !found {
			return nil, fmt.Errorf("approved tool %q is not advertised by upstream", name)
		}
	}

	transport := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20, PropagateRequestCancellation: true,
	})
	protected := http.NewCrossOriginProtection().Handler(transport)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+opts.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="alloy-node"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		protected.ServeHTTP(w, r)
	}), nil
}

func toolError(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// BearerClient adds a node credential only to the exact configured endpoint.
// Redirects are rejected so a server cannot send this credential elsewhere.
func BearerClient(endpoint, token string) *http.Client {
	return &http.Client{
		Transport:     bearerTransport{endpoint: endpoint, token: token},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type bearerTransport struct{ endpoint, token string }

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != t.endpoint {
		return nil, errors.New("node credential endpoint mismatch")
	}

	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)

	return http.DefaultTransport.RoundTrip(clone)
}
