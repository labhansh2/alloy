package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alloy/provider"
)

func TestProviderAndClientRegistration(t *testing.T) {
	srv := New()
	auth, _ := provider.BearerToken("server-secret")
	config := provider.Config{BaseURL: "https://api.example.com/v1", Auth: auth}

	for _, name := range []string{"", ".", "..", "bad/name"} {
		if err := srv.AddProvider(name, config); err == nil {
			t.Errorf("invalid provider name %q accepted", name)
		}
	}

	if err := srv.AddProvider("test", config); err != nil {
		t.Fatal(err)
	}

	if err := srv.AddProvider("test", config); err == nil {
		t.Fatal("duplicate provider accepted")
	}

	client := Client{Token: strings.Repeat("a", 32), Providers: []string{"missing"}}
	if err := srv.AddClient("client", client); err == nil {
		t.Fatal("unknown provider grant accepted")
	}

	client.Providers = []string{"test"}
	if err := srv.AddClient("client", client); err != nil {
		t.Fatal(err)
	}

	client.Providers[0] = "changed"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+client.Token)

	name, ok := srv.authenticate(r)
	if !ok || name != "client" {
		t.Fatal("registered client was not authenticated")
	}

	if err := srv.AddClient("duplicate-token", client); err == nil {
		t.Fatal("duplicate token accepted")
	}

	srv.mu.Lock()

	for digest, binding := range srv.clients {
		if !binding.providers["test"] || binding.providers["changed"] {
			t.Error("grants alias caller memory")
		}

		binding.expiresAt = time.Now().Add(-time.Second)
		srv.clients[digest] = binding
	}

	srv.mu.Unlock()

	if _, ok := srv.authenticate(r); ok {
		t.Fatal("expired client authenticated")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = srv.Run(ctx)

	if err := srv.AddProvider("later", config); err == nil {
		t.Fatal("late registration accepted")
	}

	if err := srv.AddClient("later", Client{Token: strings.Repeat("b", 32)}); err == nil {
		t.Fatal("late client registration accepted")
	}
}

func TestManagedProviderPreflightDoesNotOpenListenerOrLogin(t *testing.T) {
	srv := New()
	loginCalled := false

	auth, _ := provider.BearerToken("upstream-key")
	if err := srv.AddProvider("managed", provider.Config{
		BaseURL: "https://api.example.com/v1", Auth: auth,
		CheckAuth: func(context.Context) error { return errors.New("run with --login") },
		Login:     func(context.Context, provider.LoginOptions) error { loginCalled = true; return nil },
	}); err != nil {
		t.Fatal(err)
	}

	err := srv.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--login") || srv.Addr() != "" || loginCalled {
		t.Fatal("failed auth opened a listener, started login, or lost setup guidance")
	}

	select {
	case <-srv.Ready():
		t.Fatal("failed preflight reported readiness")
	default:
	}
}

func TestExplicitLoginDoesNotRaceServerStart(t *testing.T) {
	srv := New()
	entered := make(chan struct{})
	release := make(chan struct{})

	auth, _ := provider.BearerToken("upstream-key")
	if err := srv.AddProvider("managed", provider.Config{
		BaseURL: "https://api.example.com/v1", Auth: auth,
		Login: func(ctx context.Context, options provider.LoginOptions) error {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- srv.Login(ctx, provider.LoginOptions{}) }()

	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("login did not start")
	}

	if err := srv.Run(ctx); err == nil || srv.Addr() != "" {
		t.Error("server started while login was active")
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestInvalidClientConfiguration(t *testing.T) {
	for _, config := range []Client{
		{},
		{Token: "short"},
		{Token: strings.Repeat("a", 32) + "\n"},
		{Token: strings.Repeat("a", 32), ExpiresAt: time.Now().Add(-time.Hour)},
	} {
		if err := New().AddClient("client", config); err == nil {
			t.Error("invalid client accepted")
		}
	}
}
