package openai

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alloy/provider"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/oauth2"
)

type memoryStore struct {
	mu    sync.Mutex
	saved *credential
	host  string
}

func (s *memoryStore) locked(ctx context.Context, work func(credentialRecord) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return work(s)
}

func (s *memoryStore) load() (*credential, error) {
	if s.saved == nil {
		return nil, os.ErrNotExist
	}

	data, _ := json.Marshal(s.saved)

	var copy credential

	_ = json.Unmarshal(data, &copy)

	return &copy, nil
}

func (s *memoryStore) save(value *credential) error {
	data, _ := json.Marshal(value)

	var copy credential

	_ = json.Unmarshal(data, &copy)
	s.saved = &copy

	return nil
}

func (s *memoryStore) hostID() (string, error) {
	if s.host == "" {
		s.host = "urn:uuid:12345678-1234-4234-8234-123456789abc"
	}

	return s.host, nil
}

func TestMissingAuthAndTokenLogin(t *testing.T) {
	m := newManager("test")
	m.store = &memoryStore{}
	m.getenv = func(string) string { return "" }
	m.output = func(string) {}
	m.secret = func() (string, error) { return "saved-api-key", nil }
	ctx := context.Background()

	if err := m.check(ctx); err == nil || !strings.Contains(err.Error(), "--login") {
		t.Fatal("missing credentials did not give login guidance")
	}

	m.getenv = func(string) string { return "environment-api-key" }
	if token, err := m.accessToken(ctx); err != nil || token != "environment-api-key" {
		t.Fatal("explicit environment key did not work")
	}

	if err := m.login(ctx, provider.LoginOptions{Method: "token"}); err != nil {
		t.Fatal(err)
	}

	if token, err := m.accessToken(ctx); err != nil || token != "saved-api-key" {
		t.Fatal("saved login did not take precedence over the environment")
	}
}

type oauthFixture struct {
	server                            *httptest.Server
	signer                            jose.Signer
	mu                                sync.Mutex
	nonce, challenge, subject         string
	tokenAudience, tokenNonce, scopes string
	tokenIssuer                       string
	badSignature                      bool
	expired                           bool
	refreshes                         atomic.Int32
	exchanges                         atomic.Int32
}

func newOAuthFixture(t *testing.T) *oauthFixture {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "test-key"}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	f := &oauthFixture{signer: signer, subject: "test-subject", scopes: planScopes}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize",
				"token_endpoint": f.server.URL + "/token", "jwks_uri": f.server.URL + "/jwks",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("resource") != apiURL || r.Form.Get("client_id") != "oaiapp_test" || r.Form.Get("client_secret") != "" {
				t.Error("incorrect resource, issued client ID, or public-client auth")
			}

			if r.Form.Get("grant_type") == "refresh_token" {
				f.refreshes.Add(1)

				if r.Form.Get("refresh_token") != "original-refresh" {
					t.Error("stale refresh token")
				}

				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "renewed-access", "refresh_token": "rotated-refresh", "token_type": "Bearer", "expires_in": 3600})

				return
			}

			f.exchanges.Add(1)

			f.mu.Lock()
			defer f.mu.Unlock()

			challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(challenge[:]) != f.challenge {
				t.Error("PKCE verifier mismatch")
			}

			audience, nonce := f.tokenAudience, f.tokenNonce
			if audience == "" {
				audience = "oaiapp_test"
			}

			if nonce == "" {
				nonce = f.nonce
			}

			issuer, expiry := f.server.URL, time.Now().Add(time.Hour)
			if f.tokenIssuer != "" {
				issuer = f.tokenIssuer
			}

			if f.expired {
				expiry = time.Now().Add(-time.Hour)
			}

			token, err := jwt.Signed(f.signer).Claims(map[string]any{
				"iss": issuer, "sub": f.subject, "aud": audience,
				"exp": expiry.Unix(), "iat": time.Now().Unix(), "nonce": nonce,
			}).Serialize()
			if err != nil {
				t.Error(err)
				return
			}

			if f.badSignature {
				parts := strings.Split(token, ".")
				parts[2] = base64.RawURLEncoding.EncodeToString([]byte("invalid signature"))
				token = strings.Join(parts, ".")
			}

			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "browser-access", "refresh_token": "original-refresh", "id_token": token,
				"scope": f.scopes, "token_type": "Bearer", "expires_in": 3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)

	return f
}

func (f *oauthFixture) manager(t *testing.T) *manager {
	t.Helper()

	m := newManager("test")
	m.store = &memoryStore{}
	m.issuer = f.server.URL
	m.endpoint = oauth2.Endpoint{AuthURL: f.server.URL + "/authorize", TokenURL: f.server.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}
	m.getenv = func(string) string { return "" }
	m.output = func(string) {}
	m.open = func(ctx context.Context, authorize string) error {
		u, err := url.Parse(authorize)
		if err != nil {
			return err
		}

		q := u.Query()
		if q.Get("client_id") != "dynamic_agent_client" && q.Get("client_id") != "oaiapp_test" {
			t.Error("incorrect registration ID")
		}

		if q.Get("client_id") == "dynamic_agent_client" && q.Get("agent_name_hint") != "Alloy" {
			t.Error("missing actual app name")
		}

		if q.Get("resource") != apiURL || q.Get("ext_agent_host_id") == "" || q.Get("code_challenge_method") != "S256" {
			t.Error("missing OAuth binding")
		}

		f.mu.Lock()
		f.nonce, f.challenge = q.Get("nonce"), q.Get("code_challenge")
		f.mu.Unlock()

		callback, err := url.Parse(q.Get("redirect_uri"))
		if err != nil {
			return err
		}

		callback.RawQuery = url.Values{"state": {q.Get("state")}, "code": {"test-code"}, "client_id": {"oaiapp_test"}}.Encode()
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, callback.String(), nil)

		response, err := m.client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		_, _ = io.Copy(io.Discard, response.Body)

		return nil
	}

	return m
}

func TestBrowserLoginAndReauthorization(t *testing.T) {
	f := newOAuthFixture(t)
	m := f.manager(t)

	ctx := context.Background()
	if err := m.login(ctx, provider.LoginOptions{}); err != nil {
		t.Fatal(err)
	}

	if token, err := m.accessToken(ctx); err != nil || token != "browser-access" {
		t.Fatalf("browser auth: %v", err)
	}

	if err := m.login(ctx, provider.LoginOptions{}); err != nil {
		t.Fatal(err)
	}

	if f.exchanges.Load() != 2 {
		t.Fatal("returning login did not exchange using issued registration")
	}
}

func TestRejectsInvalidBrowserIdentityAndScopes(t *testing.T) {
	for _, kind := range []string{"audience", "nonce", "scopes", "subject", "issuer", "signature", "expiry"} {
		t.Run(kind, func(t *testing.T) {
			f := newOAuthFixture(t)

			m := f.manager(t)
			if kind == "subject" {
				if err := m.login(context.Background(), provider.LoginOptions{}); err != nil {
					t.Fatal(err)
				}
			}

			f.mu.Lock()

			switch kind {
			case "audience":
				f.tokenAudience = "another-client"
			case "nonce":
				f.tokenNonce = "wrong-nonce"
			case "issuer":
				f.tokenIssuer = "https://another-issuer.example"
			case "signature":
				f.badSignature = true
			case "expiry":
				f.expired = true
			case "scopes":
				f.scopes = "openid profile email"
			case "subject":
				f.subject = "another-account"
			}

			f.mu.Unlock()

			if err := m.login(context.Background(), provider.LoginOptions{}); err == nil {
				t.Fatal("invalid authorization was accepted")
			}

			saved, err := m.store.(*memoryStore).load()
			if kind != "subject" && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid login saved credentials")
			}

			if kind == "subject" && (err != nil || saved.Subject != "test-subject") {
				t.Fatal("existing account was overwritten")
			}
		})
	}
}

func TestInvalidStateDoesNotConsumeLogin(t *testing.T) {
	f := newOAuthFixture(t)
	m := f.manager(t)
	valid := m.open

	m.open = func(ctx context.Context, authorize string) error {
		u, _ := url.Parse(authorize)
		callback, _ := url.Parse(u.Query().Get("redirect_uri"))
		callback.RawQuery = "state=wrong&code=invalid&client_id=oaiapp_attacker"

		response, err := m.client.Get(callback.String())
		if err != nil {
			return err
		}

		_ = response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Error("invalid state was accepted")
		}

		return valid(ctx, authorize)
	}
	if err := m.login(context.Background(), provider.LoginOptions{}); err != nil {
		t.Fatal(err)
	}

	if f.exchanges.Load() != 1 {
		t.Fatal("unverified code reached token exchange")
	}
}

func TestRefreshSerializesAndPersistsRotatingTokens(t *testing.T) {
	f := newOAuthFixture(t)

	m := f.manager(t)
	if err := m.login(context.Background(), provider.LoginOptions{}); err != nil {
		t.Fatal(err)
	}

	store := m.store.(*memoryStore)
	store.saved.Token.Expiry = time.Now().Add(-time.Second)
	other := f.manager(t)
	other.store = store

	var wg sync.WaitGroup
	for _, manager := range []*manager{m, other} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if token, err := manager.accessToken(context.Background()); err != nil || token != "renewed-access" {
				t.Errorf("refresh failed: %v", err)
			}
		}()
	}

	wg.Wait()

	if f.refreshes.Load() != 1 || store.saved.Token.RefreshToken != "rotated-refresh" {
		t.Fatal("refresh raced or rotation was not saved")
	}
}

func TestBrowserCancellationDoesNotSaveCredentials(t *testing.T) {
	m := newManager("test")
	m.store = &memoryStore{}
	m.output = func(string) {}
	ctx, cancel := context.WithCancel(context.Background())
	m.open = func(context.Context, string) error { cancel(); return nil }

	defer cancel()

	if err := m.login(ctx, provider.LoginOptions{}); err == nil {
		t.Fatal("cancellation was ignored")
	}

	if _, err := m.store.(*memoryStore).load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled login saved credentials")
	}
}

func TestNativeClientUsesAlloyAuth(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer alloy-token" {
			t.Error("native client connection settings were not applied")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[],"object":"list"}`)
	}))
	defer upstream.Close()

	client := Client(provider.NewConnection(upstream.URL+"/", "alloy-token"))
	if _, err := client.Models.List(context.Background()); err != nil {
		t.Fatal(err)
	}
}
