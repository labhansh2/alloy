// Package providerproxy serves an authorized, fixed-upstream HTTP connection.
// Authentication and grants are checked by the server before invoking it.
package providerproxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"alloy/provider"
)

type Gateway struct {
	proxy     *httputil.ReverseProxy
	transport *http.Transport
	timeout   time.Duration
	maxBytes  int64
}

func New(config provider.Config) (*Gateway, error) {
	u, err := url.Parse(config.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("invalid provider base URL")
	}

	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return nil, errors.New("provider requires HTTPS or explicit loopback HTTP")
	}

	if !safePath(u.Path) || config.Auth == nil || config.Timeout < 0 || config.MaxRequestBytes < 0 {
		return nil, errors.New("invalid provider path, authentication, or limits")
	}

	if config.Timeout == 0 {
		config.Timeout = time.Minute
	}

	if config.MaxRequestBytes == 0 {
		config.MaxRequestBytes = 1 << 20
	}

	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  config.Timeout,
		IdleConnTimeout:        30 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DisableCompression:     true,
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Path = "/" + r.In.PathValue("path")
			r.Out.URL.RawPath = ""
			r.SetURL(u)

			// Remove Alloy authority, ambient cookies, forwarding identity, and
			// common auth headers before the server authenticator runs. Native
			// service-specific policy belongs in the registered authenticator.
			for _, header := range []string{
				"Authorization", "Proxy-Authorization", "Cookie", "X-Api-Key", "Api-Key", "X-Goog-Api-Key",
				"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto",
			} {
				r.Out.Header.Del(header)
			}
		},
		Transport:     authenticatedTransport{base: transport, auth: config.Auth},
		FlushInterval: -1,
		ErrorLog:      log.New(io.Discard, "", 0),
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// Authenticator/transport errors may contain secrets. Do not echo them.
			http.Error(w, "provider request failed", http.StatusBadGateway)
		},
		ModifyResponse: func(response *http.Response) error {
			switch response.StatusCode {
			case http.StatusMultipleChoices, http.StatusMovedPermanently, http.StatusFound,
				http.StatusSeeOther, http.StatusUseProxy,
				http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
				return errors.New("provider redirects are not supported")
			}

			response.Header.Del("Set-Cookie")
			response.Header.Del("WWW-Authenticate")

			return nil
		},
	}

	return &Gateway{proxy: proxy, transport: transport, timeout: config.Timeout, maxBytes: config.MaxRequestBytes}, nil
}

func (g *Gateway) Close() {
	g.transport.CloseIdleConnections()
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect || r.Header.Get("Upgrade") != "" || !safePath(r.PathValue("path")) {
		http.Error(w, "unsupported provider request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), g.timeout)
	defer cancel()

	deadline := time.Now().Add(g.timeout)
	_ = http.NewResponseController(w).SetReadDeadline(deadline)
	_ = http.NewResponseController(w).SetWriteDeadline(deadline)

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, g.maxBytes))
	if err != nil {
		http.Error(w, "provider request body is invalid or too large", http.StatusRequestEntityTooLarge)
		return
	}

	r = r.Clone(ctx)
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	// Permit streaming up to the explicit request deadline, rather than the
	// server's shorter default write timeout. Client cancellation still applies.
	g.proxy.ServeHTTP(w, r)
}

func safePath(path string) bool {
	if strings.ContainsAny(path, "%\\") {
		return false
	}

	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return false
		}
	}

	return true
}

type authenticatedTransport struct {
	base http.RoundTripper
	auth provider.Authenticator
}

func (t authenticatedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.auth(r); err != nil {
		return nil, errors.New("upstream authentication unavailable")
	}

	return t.base.RoundTrip(r)
}
