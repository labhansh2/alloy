package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alloy/internal/prototype/nodewire"
)

func TestDefaultStateDirPreservesExistingConsent(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "alloy", "prototype-server")

	legacy := filepath.Join(root, "alloy", "prototype-notion")
	if got := defaultStateDir(root); got != current {
		t.Fatalf("new default: %s", got)
	}

	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}

	if got := defaultStateDir(root); got != legacy {
		t.Fatalf("legacy consent not reused: %s", got)
	}

	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatal(err)
	}

	if got := defaultStateDir(root); got != current {
		t.Fatalf("explicit current state not preferred: %s", got)
	}
}

func TestSharedCLIAndCodexRoutesKeepDistinctCredentials(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "private")
	codeHome := t.TempDir()
	t.Setenv("CODEX_HOME", codeHome)

	if err := os.WriteFile(filepath.Join(codeHome, "auth.json"), []byte(`{"tokens":{"access_token":"fake-upstream-for-local-test"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()

	host := httptest.NewServer(mux)
	defer host.Close()

	if err := registerCLI(mux, host.URL, stateDir); err != nil {
		t.Fatal(err)
	}

	if err := registerCodex(mux, host.URL, stateDir, "browser-inference-token"); err != nil {
		t.Fatal(err)
	}

	var (
		cli struct {
			Endpoint string `json:"endpoint"`
			Token    string `json:"token"`
		}
		codex struct {
			BaseURL string `json:"baseUrl"`
			Token   string `json:"token"`
		}
	)

	read := func(name string, out any) {
		t.Helper()

		data, err := os.ReadFile(filepath.Join(stateDir, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
	read("cli-node-connection.json", &cli)
	read("codex-node-connection.json", &codex)

	if codex.Token == cli.Token || codex.Token == "browser-inference-token" {
		t.Fatal("node grants share a credential")
	}

	for _, tc := range []struct {
		path, token string
		status      int
	}{
		{"/node", codex.Token, 401},
		{"/codex/not-a-route", cli.Token, 401},
		// Authenticated non-response paths return 404 without sending anything
		// upstream. No real model request is made by this test.
		{"/codex/not-a-route", codex.Token, 404},
		{"/codex/not-a-route", "browser-inference-token", 404},
	} {
		method := http.MethodPost
		if tc.path == "/node" {
			method = http.MethodGet
		}

		r := httptest.NewRequest(method, tc.path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+tc.token)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)

		if w.Code != tc.status {
			t.Errorf("%s: got %d, want %d", tc.path, w.Code, tc.status)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	node, err := nodewire.Dial(ctx, cli.Endpoint, "prototype-node", cli.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()

	if err := node.Publish(ctx, map[string]string{"kind": "merged-server-test"}); err != nil {
		t.Fatal(err)
	}
}
