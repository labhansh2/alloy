package mcpoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRefreshIsPersistedAndReused(t *testing.T) {
	calls := 0

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}

		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" {
			t.Error("unexpected refresh request")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`))
	}))
	defer provider.Close()

	store := Store{Path: filepath.Join(t.TempDir(), "private", "oauth.json")}
	config := &oauth2.Config{ClientID: "test-client", Endpoint: oauth2.Endpoint{TokenURL: provider.URL, AuthStyle: oauth2.AuthStyleInParams}}

	source, err := store.NewTokenSource(context.Background(), config, &oauth2.Token{AccessToken: "old-access", RefreshToken: "old-refresh", Expiry: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	token, err := source.Token()
	if err != nil || token.AccessToken != "new-access" {
		t.Fatalf("refresh failed: %v", err)
	}

	info, err := os.Stat(store.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v", err)
	}

	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}

	var saved savedConnection
	if json.Unmarshal(data, &saved) != nil || saved.Token.RefreshToken != "new-refresh" {
		t.Fatal("rotated refresh token was not persisted")
	}

	loaded, err := store.Load(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := loaded.Token(); err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Fatalf("cached access token not reused; refresh calls=%d", calls)
	}
}

func TestOriginClientRejectsUnapprovedOrigins(t *testing.T) {
	client, err := OriginClient("https://approved.example")
	if err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range []string{"https://other.example/token", "http://approved.example/token"} {
		if _, err := client.Get(endpoint); err == nil {
			t.Fatalf("unapproved endpoint accepted: %s", endpoint)
		}
	}
}
