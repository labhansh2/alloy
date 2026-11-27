// Package provider describes native HTTP service connections. Alloy manages
// access and upstream authentication without translating inference APIs.
package provider

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Authenticator applies upstream authentication to a server-owned request.
// Implementations are trusted server code and must be safe for concurrent use.
// They may use request.Context() for supported token refresh or cancellation.
type Authenticator func(request *http.Request) error

// Config registers an operator-approved upstream, not an Alloy model catalog.
// HTTPS is required except for explicitly configured loopback HTTP services.
type Config struct {
	BaseURL string
	Auth    Authenticator
	// CheckAuth validates managed credentials before the server opens a listener.
	// Nil leaves credential validation to the explicit authenticator.
	CheckAuth func(context.Context) error
	// Login performs explicit provider setup. It is never called by normal Run.
	Login func(context.Context, LoginOptions) error
	// Timeout bounds an entire upstream request, including streamed output.
	// Zero selects one minute.
	Timeout time.Duration
	// MaxRequestBytes bounds buffered request bodies. Zero selects 1 MiB.
	MaxRequestBytes int64
}

// LoginOptions selects an explicit provider login method. Providers document
// their supported methods; a zero value selects their browser flow.
type LoginOptions struct {
	Method string
}

// HTTPClient supplies a native client with no environment proxy, no redirects,
// and a one-minute request timeout. It contains no provider or Alloy secrets.
func (c Connection) HTTPClient() *http.Client {
	return &http.Client{
		Transport: nativeTransport,
		Timeout:   time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("provider redirects are not supported")
		},
	}
}

var nativeTransport = &http.Transport{Proxy: nil, IdleConnTimeout: 30 * time.Second}

// BearerToken creates server-side bearer authentication, for example an API key.
// It does not implement browser login or imply subscription-backed API access.
func BearerToken(token string) (Authenticator, error) {
	if token == "" {
		return nil, errors.New("provider bearer token must be nonempty and contain no whitespace")
	}

	for _, c := range token {
		if c < 33 || c > 126 {
			return nil, errors.New("invalid provider bearer token")
		}
	}

	return func(request *http.Request) error {
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}, nil
}

// Connection contains native client settings for an authorized Alloy gateway.
// Its token is an Alloy credential, never the upstream provider credential.
// Treat it as sensitive; Token deliberately exposes authority for native SDKs.
type Connection struct {
	baseURL string
	token   string
}

// NewConnection packages native settings. It does not validate access; normal
// callers should obtain a connection through node.Provider instead.
func NewConnection(baseURL, token string) Connection {
	return Connection{baseURL: baseURL, token: token}
}

func (c Connection) BaseURL() string {
	return c.baseURL
}

func (c Connection) Token() string {
	return c.token
}

func (c Connection) String() string {
	return "provider.Connection{credentials:redacted}"
}

func (c Connection) GoString() string {
	return c.String()
}
