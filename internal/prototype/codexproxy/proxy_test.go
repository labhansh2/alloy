package codexproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthBroker(t *testing.T) {
	calls := 0
	handler := newHandler("host-secret", "host-account", "node-token", transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://chatgpt.com/backend-api/codex/responses" {
			t.Errorf("unexpected upstream: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer host-secret" || r.Header.Get("ChatGPT-Account-ID") != "host-account" {
			t.Error("host credentials were not injected")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("streamed response"))}, nil
	}))
	for _, tc := range []struct {
		path, token, method string
		status              int
	}{
		{"/codex/responses", "", "POST", 401},
		{"/codex/responses", "wrong-token", "POST", 401},
		{"/codex/other", "node-token", "POST", 404},
		{"/codex/responses", "node-token", "GET", 404},
		{"/codex/responses?ignored=true", "node-token", "POST", 200},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("ChatGPT-Account-ID", "spoofed-account")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, w.Code, tc.status)
		}
		if strings.Contains(w.Body.String(), "host-secret") {
			t.Error("host secret leaked in response")
		}
	}
	if calls != 1 {
		t.Errorf("upstream calls = %d, want 1", calls)
	}
}
