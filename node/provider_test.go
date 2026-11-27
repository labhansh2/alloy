package node_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alloy/node"
	"alloy/provider"
	"alloy/server"
)

const clientToken = "alloy-test-client-token-0123456789abcdef"

func TestNativeProviderConnection(t *testing.T) {
	const upstreamKey = "upstream-test-secret"

	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/v1/responses" || r.URL.RawQuery != "native=value" {
			t.Errorf("native target changed: %s", r.URL)
		}

		if r.Header.Get("Authorization") != "Bearer "+upstreamKey {
			t.Error("upstream did not receive server-side authentication")
		}

		for _, header := range []string{"Cookie", "X-Api-Key", "Forwarded"} {
			if r.Header.Get(header) != "" {
				t.Errorf("client header %s leaked upstream", header)
			}
		}

		if r.Header.Get("Openai-Beta") != "native-feature" {
			t.Error("native feature header was lost")
		}

		if r.Header.Get("Openai-Project") != "native-project" {
			t.Error("native service header was lost")
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"model":"new-model","input":"hello"}` {
			t.Errorf("native request changed: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "upstream-session=secret")
		_, _ = io.WriteString(w, `{"output":"native result"}`)
	}))
	defer upstream.Close()

	auth, err := provider.BearerToken(upstreamKey)
	if err != nil {
		t.Fatal(err)
	}

	srv := providerServer(t, upstream.URL+"/v1", auth)
	n := authenticatedNode(t, srv, clientToken)
	start(t, n.Run, n.Ready())

	connection, err := n.Provider(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}

	if connection.Token() != clientToken || strings.Contains(fmt.Sprintf("%+v %#v", connection, connection), clientToken) {
		t.Fatal("connection returned the wrong token or leaked it through formatting")
	}

	r, err := http.NewRequest(http.MethodPost, connection.BaseURL()+"responses?native=value", strings.NewReader(`{"model":"new-model","input":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Authorization", "Bearer "+connection.Token())
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Cookie", "local=secret")
	r.Header.Set("X-Api-Key", "client-key")
	r.Header.Set("Forwarded", "for=client")
	r.Header.Set("Openai-Project", "native-project")
	r.Header.Set("Openai-Beta", "native-feature")

	response := send(t, r)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != `{"output":"native result"}` {
		t.Fatalf("native response changed: status=%d body=%s err=%v", response.StatusCode, body, err)
	}

	if response.Header.Get("Set-Cookie") != "" || calls.Load() != 1 {
		t.Fatal("unexpected cookie or upstream call count")
	}

	if !srv.RevokeClient("allowed") {
		t.Fatal("could not revoke client")
	}

	revoked := providerRequest(t, connection.BaseURL()+"responses", clientToken)
	defer revoked.Body.Close()

	if revoked.StatusCode != http.StatusUnauthorized || calls.Load() != 1 {
		t.Fatal("revoked connection retained access")
	}
}

func TestProviderDenialsDoNotReachUpstream(t *testing.T) {
	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	auth, _ := provider.BearerToken("upstream-secret")

	srv := server.New()
	if err := srv.AddProvider("test", provider.Config{BaseURL: upstream.URL, Auth: auth}); err != nil {
		t.Fatal(err)
	}

	if err := srv.AddClient("denied", server.Client{Token: clientToken}); err != nil {
		t.Fatal(err)
	}

	start(t, srv.Run, srv.Ready())

	for _, tc := range []struct {
		token  string
		status int
	}{
		{"", http.StatusUnauthorized},
		{"incorrect-token", http.StatusUnauthorized},
		{clientToken, http.StatusForbidden},
	} {
		for _, suffix := range []string{"", "/http/responses"} {
			response := providerRequest(t, "http://"+srv.Addr()+"/alloy/providers/test"+suffix, tc.token)
			_ = response.Body.Close()

			if response.StatusCode != tc.status {
				t.Errorf("denial status=%d; want %d", response.StatusCode, tc.status)
			}
		}
	}

	n := authenticatedNode(t, srv, clientToken)
	if _, err := n.Provider(context.Background(), "test"); err == nil {
		t.Fatal("denied provider discovery succeeded")
	}

	if calls.Load() != 0 {
		t.Fatal("denied request reached upstream")
	}
}

func TestInvalidClientCannotHandshake(t *testing.T) {
	srv := server.New()
	start(t, srv.Run, srv.Ready())
	n := authenticatedNode(t, srv, clientToken)

	if err := n.Run(context.Background()); err == nil {
		t.Fatal("unknown client authenticated")
	}

	assertNotReady(t, n)
}

func TestProviderAuthFailureIsSanitized(t *testing.T) {
	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer upstream.Close()

	srv := providerServer(t, upstream.URL, func(*http.Request) error {
		return errors.New("refresh failed: seeded-secret")
	})

	response := providerRequest(t, "http://"+srv.Addr()+"/alloy/providers/test/http/responses", clientToken)
	defer response.Body.Close()

	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusBadGateway || strings.Contains(string(body), "seeded-secret") || calls.Load() != 0 {
		t.Fatalf("auth failure leaked or reached upstream: %d %s", response.StatusCode, body)
	}
}

func TestProviderStreamsNativeOutput(t *testing.T) {
	release := make(chan struct{})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()

		select {
		case <-release:
			_, _ = io.WriteString(w, "data: second\n\n")
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	defer close(release)

	auth, _ := provider.BearerToken("upstream-secret")
	srv := providerServer(t, upstream.URL, auth)

	response := providerRequest(t, "http://"+srv.Addr()+"/alloy/providers/test/http/responses", clientToken)
	defer response.Body.Close()

	first := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(response.Body, first); err != nil || string(first) != "data: first\n\n" {
		t.Fatalf("stream buffered or altered: %q %v", first, err)
	}
}

func providerServer(t *testing.T, upstream string, auth provider.Authenticator) *server.Server {
	t.Helper()

	srv := server.New()
	if err := srv.AddProvider("test", provider.Config{BaseURL: upstream, Auth: auth}); err != nil {
		t.Fatal(err)
	}

	if err := srv.AddClient("allowed", server.Client{Token: clientToken, Providers: []string{"test"}}); err != nil {
		t.Fatal(err)
	}

	start(t, srv.Run, srv.Ready())

	return srv
}

func authenticatedNode(t *testing.T, srv *server.Server, token string) *node.Node {
	t.Helper()

	n, err := node.New(node.Config{Name: "descriptive-name", ServerURL: "http://" + srv.Addr(), ClientToken: token})
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func providerRequest(t *testing.T, endpoint, token string) *http.Response {
	t.Helper()

	r, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}

	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	return send(t, r)
}

func send(t *testing.T, r *http.Request) *http.Response {
	t.Helper()

	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}

	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}

	return response
}
