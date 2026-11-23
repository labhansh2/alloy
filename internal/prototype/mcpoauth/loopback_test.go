package mcpoauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

func TestCallbackRejectsWrongStateAndReplay(t *testing.T) {
	f := NewLoopback()
	p := &pendingLogin{url: "https://provider.example/authorize?state=expected", state: "expected", result: make(chan loginResult, 1)}
	f.pending = p

	request := func(path string) int {
		w := httptest.NewRecorder()
		f.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

		return w.Code
	}
	if code := request("/oauth/callback?state=wrong&code=private"); code != 400 {
		t.Fatalf("wrong state: %d", code)
	}

	if code := request("/oauth/callback?state=expected&code=private"); code != 200 {
		t.Fatalf("valid callback: %d", code)
	}

	if got := <-p.result; got.auth == nil || got.auth.Code != "private" {
		t.Fatal("missing callback result")
	}

	if code := request("/oauth/callback?state=expected&code=private"); code != 409 {
		t.Fatalf("replay: %d", code)
	}
}

func TestFetchCancels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	f := NewLoopback()
	if _, err := f.Fetch(ctx, &auth.AuthorizationArgs{URL: "https://provider.example/authorize?state=random"}); err == nil {
		t.Fatal("expected cancellation")
	}

	if f.pending != nil {
		t.Fatal("pending OAuth state survived cancellation")
	}
}
