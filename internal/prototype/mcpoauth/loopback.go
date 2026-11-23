// Package mcpoauth provides reusable OAuth plumbing for a server-owned MCP
// connection. Providers, routes, and approved tools are configured by examples.
package mcpoauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

// Loopback is a single-flight, one-use browser callback. Mount Handler on the
// already bound loopback server before opening an upstream MCP connection.
type Loopback struct {
	StartPath string
	mu        sync.Mutex
	pending   *pendingLogin
}

type pendingLogin struct {
	url, state string
	result     chan loginResult
}

type loginResult struct {
	auth *auth.AuthorizationResult
	err  error
}

func NewLoopback() *Loopback { return &Loopback{StartPath: "/oauth/start/" + rand.Text()} }

// Fetch is passed directly to the official MCP SDK, which generates PKCE and
// validates the state again during authorization-code exchange.
func (f *Loopback) Fetch(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	u, err := url.Parse(args.URL)
	if err != nil || u.Scheme != "https" || u.Query().Get("state") == "" {
		return nil, errors.New("invalid authorization URL")
	}
	p := &pendingLogin{url: args.URL, state: u.Query().Get("state"), result: make(chan loginResult, 1)}
	f.mu.Lock()
	if f.pending != nil {
		f.mu.Unlock()
		return nil, errors.New("authorization already in progress")
	}
	f.pending = p
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		if f.pending == p {
			f.pending = nil
		}
		f.mu.Unlock()
	}()
	select {
	case result := <-p.result:
		return result.auth, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *Loopback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.pending
	if p == nil {
		http.Error(w, "No pending login. Start the server and try again.", 409)
		return
	}
	if r.URL.Path == f.StartPath {
		http.Redirect(w, r, p.url, http.StatusFound)
		return
	}
	if r.URL.Path != "/oauth/callback" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(p.state)) != 1 {
		http.Error(w, "Invalid OAuth state.", 400)
		return
	}
	if q.Get("error") != "" {
		p.result <- loginResult{err: errors.New("OAuth consent was denied or failed")}
		f.pending = nil
		http.Error(w, "Authorization was not completed.", 400)
		return
	}
	if q.Get("code") == "" {
		http.Error(w, "Missing authorization code.", 400)
		return
	}
	p.result <- loginResult{auth: &auth.AuthorizationResult{Code: q.Get("code"), State: q.Get("state"), Iss: q.Get("iss")}}
	f.pending = nil
	_, _ = w.Write([]byte("Authorization code received. Return to the server terminal to confirm connection. You may close this tab.\n"))
}
