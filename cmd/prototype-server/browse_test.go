package main

import (
	"encoding/json"
	"testing"
)

func TestBrowseSessionPolicy(t *testing.T) {
	filter := pinBrowseSession("server-session")
	for _, raw := range []string{
		`{"code":"return page.url()","session":"other-session"}`,
		`{"code":"return page.url()","browser":"other-browser"}`,
		`{"code":"return page.url()","targetUrl":"private.example"}`,
		`{"code":"return page.url()","targetIndex":0}`,
		`{"code":""}`, `null`, `[]`,
	} {
		if _, err := filter("execute", json.RawMessage(raw)); err == nil {
			t.Errorf("policy accepted %s", raw)
		}
	}

	if _, err := filter("session_allowlist", json.RawMessage(`{"allowedOrigins":["*"]}`)); err == nil {
		t.Error("policy accepted a different tool")
	}

	data, err := filter("execute", json.RawMessage(`{"code":"return page.url()","timeoutMs":0}`))
	if err != nil {
		t.Fatal(err)
	}

	var args map[string]any
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}

	if args["session"] != "server-session" || args["timeoutMs"] != float64(45000) {
		t.Fatalf("session/deadline not constrained: %s", data)
	}
}
