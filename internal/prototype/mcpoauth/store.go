package mcpoauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// Store keeps a single server-owned OAuth connection in a private directory.
// This is permission-protected plaintext, not an encrypted vault/keychain.
// The application must prevent multiple server processes sharing this store.
type Store struct{ Path string }

type savedConnection struct {
	Config oauth2.Config `json:"config"`
	Token  *oauth2.Token `json:"token"`
}

// SavePrivateJSON atomically replaces a file without exposing partially written
// credentials. Its dedicated parent directory must not be group/world readable.
func SavePrivateJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("credential directory must be private (0700)")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".alloy-credential-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// NewTokenSource is the MCP SDK hook: persist initial login and subsequent
// refreshes using oauth2's native token-source implementation.
func (s Store) NewTokenSource(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
	if err := SavePrivateJSON(s.Path, savedConnection{Config: *config, Token: token}); err != nil {
		return nil, errors.New("cannot save server OAuth connection")
	}
	return &persistedSource{source: config.TokenSource(ctx, token), config: *config, store: s, last: *token}, nil
}

func (s Store) Load(ctx context.Context, client *http.Client) (oauth2.TokenSource, error) {
	info, err := os.Lstat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("OAuth cache must be a private regular file (0600)")
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, errors.New("cannot read server OAuth cache")
	}
	var saved savedConnection
	if json.Unmarshal(data, &saved) != nil || saved.Token == nil || saved.Token.AccessToken == "" {
		return nil, errors.New("invalid server OAuth cache")
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	return &persistedSource{source: saved.Config.TokenSource(ctx, saved.Token), config: saved.Config, store: s, last: *saved.Token}, nil
}

type persistedSource struct {
	mu     sync.Mutex
	source oauth2.TokenSource
	config oauth2.Config
	store  Store
	last   oauth2.Token
}

func (s *persistedSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, errors.New("upstream OAuth refresh failed; reconnect on the server")
	}
	if token.AccessToken != s.last.AccessToken || token.RefreshToken != s.last.RefreshToken || !token.Expiry.Equal(s.last.Expiry) {
		if err := SavePrivateJSON(s.store.Path, savedConnection{Config: s.config, Token: token}); err != nil {
			return nil, errors.New("cannot persist refreshed upstream OAuth token")
		}
		s.last = *token
	}
	return token, nil
}

// OriginClient limits provider discovery, registration and token exchange to
// the operator-configured HTTPS origin. Redirects and ambient proxies are off.
func OriginClient(origin string) (*http.Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("provider origin must be HTTPS")
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	return &http.Client{
		Transport:     originTransport{origin: u, base: base},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

type originTransport struct {
	origin *url.URL
	base   http.RoundTripper
}

func (t originTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != t.origin.Scheme || r.URL.Host != t.origin.Host || r.URL.User != nil {
		return nil, errors.New("OAuth/MCP request outside approved provider origin")
	}
	return t.base.RoundTrip(r)
}
