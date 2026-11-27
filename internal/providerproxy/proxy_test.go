package providerproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alloy/provider"
)

func TestInvalidConfiguration(t *testing.T) {
	auth, _ := provider.BearerToken("server-secret")
	for _, config := range []provider.Config{
		{BaseURL: "https://api.example.com"},
		{BaseURL: "http://example.com", Auth: auth},
		{BaseURL: "http://localhost:8080", Auth: auth},
		{BaseURL: "https://user:password@example.com", Auth: auth},
		{BaseURL: "https://example.com?key=secret", Auth: auth},
		{BaseURL: "https://example.com#fragment", Auth: auth},
		{BaseURL: "https://example.com/v1/../admin", Auth: auth},
		{BaseURL: "https://example.com", Auth: auth, Timeout: -time.Second},
		{BaseURL: "https://example.com", Auth: auth, MaxRequestBytes: -1},
	} {
		if _, err := New(config); err == nil {
			t.Error("invalid configuration accepted")
		}
	}
}

func TestRequestLimitsAndPaths(t *testing.T) {
	var calls atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer upstream.Close()

	auth, _ := provider.BearerToken("server-secret")

	gateway, err := New(provider.Config{BaseURL: upstream.URL, Auth: auth, MaxRequestBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()

	for _, tc := range []struct {
		path   string
		body   string
		status int
	}{
		{"responses", "12345", http.StatusRequestEntityTooLarge},
		{"../admin", "", http.StatusBadRequest},
		{"%2e%2e/admin", "", http.StatusBadRequest},
		{"foo\\bar", "", http.StatusBadRequest},
	} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		r.SetPathValue("path", tc.path)

		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, r)

		if w.Code != tc.status {
			t.Errorf("request status=%d; want %d", w.Code, tc.status)
		}
	}

	if calls.Load() != 0 {
		t.Fatal("invalid request reached upstream")
	}
}

func TestRedirectIsNotFollowedOrReturned(t *testing.T) {
	var destinationCalls atomic.Int32

	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls.Add(1)
	}))
	defer destination.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()

	auth, _ := provider.BearerToken("server-secret")

	gateway, err := New(provider.Config{BaseURL: upstream.URL, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("path", "responses")

	w := httptest.NewRecorder()
	gateway.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway || w.Header().Get("Location") != "" || destinationCalls.Load() != 0 {
		t.Fatal("upstream redirect escaped the configured connection")
	}
}

func TestTimeoutCancelsUpstream(t *testing.T) {
	done := make(chan struct{})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		<-r.Context().Done()
	}))
	defer upstream.Close()

	auth, _ := provider.BearerToken("server-secret")

	gateway, err := New(provider.Config{BaseURL: upstream.URL, Auth: auth, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("path", "responses")

	w := httptest.NewRecorder()
	gateway.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("timeout status=%d", w.Code)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not observe cancellation")
	}
}

func TestDynamicAuthentication(t *testing.T) {
	var current atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("X-Service-Auth"))
	}))
	defer upstream.Close()

	gateway, err := New(provider.Config{
		BaseURL: upstream.URL,
		Auth: func(r *http.Request) error {
			if r.Context().Err() != nil {
				return context.Canceled
			}

			if current.Load() == 0 {
				r.Header.Set("X-Service-Auth", "first")
			} else {
				r.Header.Set("X-Service-Auth", "refreshed")
			}

			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()

	for i, want := range []string{"first", "refreshed"} {
		current.Store(int32(i))

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.SetPathValue("path", "responses")

		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, r)

		if w.Code != http.StatusOK || w.Body.String() != want {
			t.Fatalf("dynamic auth not applied: status=%d", w.Code)
		}
	}
}
