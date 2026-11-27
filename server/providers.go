package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"alloy/internal/nodesession"
	"alloy/internal/providerproxy"
	"alloy/internal/providerwire"
	"alloy/provider"
)

// Client binds a protected Alloy token to an explicit set of provider grants.
// The token must contain at least 32 non-whitespace ASCII characters; generate
// it randomly and pass it through protected configuration, never command args.
type Client struct {
	Token     string `json:"-"`
	Providers []string
	// A zero expiry lasts until revocation or process exit.
	ExpiresAt time.Time
}

func (c Client) String() string {
	return "server.Client{credentials:redacted}"
}

func (c Client) GoString() string {
	return c.String()
}

type clientBinding struct {
	name      string
	providers map[string]bool
	expiresAt time.Time
}

type providerSetup struct {
	check func(context.Context) error
	login func(context.Context, provider.LoginOptions) error
}

// AddProvider registers a fixed-upstream native HTTP connection before Run.
// Registration grants no client access. Provider credentials stay server-side.
func (s *Server) AddProvider(name string, config provider.Config) error {
	if err := nodesession.ValidateName(name); err != nil || name == "." || name == ".." {
		return errors.New("invalid provider name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.loggingIn {
		return errors.New("providers must be registered before Run")
	}

	if s.providers[name] != nil {
		return errors.New("provider already registered")
	}

	gateway, err := providerproxy.New(config)
	if err != nil {
		return err
	}

	if s.providers == nil {
		s.providers = make(map[string]*providerproxy.Gateway)
	}

	s.providers[name] = gateway
	if s.setups == nil {
		s.setups = make(map[string]providerSetup)
	}

	s.setups[name] = providerSetup{check: config.CheckAuth, login: config.Login}

	return nil
}

// Login runs the registered providers' explicit authentication setup. It must
// be invoked before Run, typically behind an application's --login flag.
func (s *Server) Login(ctx context.Context, options provider.LoginOptions) error {
	setups, err := s.providerSetups()
	if err != nil {
		return err
	}

	defer func() {
		s.mu.Lock()
		s.loggingIn = false
		s.mu.Unlock()
	}()

	names := make([]string, 0, len(setups))
	for name := range setups {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		if login := setups[name].login; login != nil {
			if err := login(ctx, options); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Server) providerSetups() (map[string]providerSetup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.loggingIn {
		return nil, errors.New("provider login must happen before Run")
	}

	s.loggingIn = true

	setups := make(map[string]providerSetup, len(s.setups))
	for name, setup := range s.setups {
		setups[name] = setup
	}

	return setups, nil
}

// CheckAuth checks registered managed providers without opening a listener or
// starting a login. Run also performs this preflight automatically.
func (s *Server) CheckAuth(ctx context.Context) error {
	s.mu.Lock()

	setups := make(map[string]providerSetup, len(s.setups))
	for name, setup := range s.setups {
		setups[name] = setup
	}

	s.mu.Unlock()

	for name, setup := range setups {
		if setup.check != nil {
			if err := setup.check(ctx); err != nil {
				return fmt.Errorf("provider %q is not ready: %w", name, err)
			}
		}
	}

	return nil
}

// AddClient registers a named client and its provider grants before Run.
// Tokens are stored as hashes, never returned by discovery or used upstream.
func (s *Server) AddClient(name string, config Client) error {
	if err := nodesession.ValidateName(name); err != nil {
		return err
	}

	if len(config.Token) < 32 || len(config.Token) > 4096 {
		return errors.New("client token must contain 32–4096 non-whitespace ASCII characters")
	}

	for _, c := range config.Token {
		if c < 33 || c > 126 {
			return errors.New("invalid client token")
		}
	}

	if !config.ExpiresAt.IsZero() && !config.ExpiresAt.After(time.Now()) {
		return errors.New("client expiry must be in the future")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.loggingIn {
		return errors.New("clients must be registered before Run")
	}

	digest := sha256.Sum256([]byte(config.Token))
	for hash, existing := range s.clients {
		if hash == digest || existing.name == name {
			return errors.New("client name or token already registered")
		}
	}

	grants := make(map[string]bool, len(config.Providers))
	for _, name := range config.Providers {
		if s.providers[name] == nil {
			return errors.New("client requests an unregistered provider")
		}

		grants[name] = true
	}

	if s.clients == nil {
		s.clients = make(map[[32]byte]clientBinding)
	}

	s.clients[digest] = clientBinding{name: name, providers: grants, expiresAt: config.ExpiresAt}

	return nil
}

// RevokeClient prevents new authenticated sessions and provider requests.
// It does not undo or cancel requests already admitted, or disconnect the
// handshake-only session, which carries no provider authority itself.
func (s *Server) RevokeClient(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for digest, client := range s.clients {
		if client.name == name {
			delete(s.clients, digest)
			return true
		}
	}

	return false
}

func (s *Server) authenticate(r *http.Request) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clientLocked(r)

	return client.name, ok
}

func (s *Server) clientLocked(r *http.Request) (clientBinding, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return clientBinding{}, false
	}

	client, ok := s.clients[sha256.Sum256([]byte(strings.TrimPrefix(header, "Bearer ")))]
	if !ok || (!client.expiresAt.IsZero() && !time.Now().Before(client.expiresAt)) {
		return clientBinding{}, false
	}

	return client, true
}

func (s *Server) providerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		http.Error(w, "browser provider access is not supported", http.StatusForbidden)
		return
	}

	name := r.PathValue("provider")

	s.mu.Lock()
	client, ok := s.clientLocked(r)
	allowed := client.providers[name]
	gateway := s.providers[name]
	s.mu.Unlock()

	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !allowed || gateway == nil {
		http.Error(w, "provider access denied", http.StatusForbidden)
		return
	}

	if r.URL.Path == providerwire.AccessPath(name) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	gateway.ServeHTTP(w, r)
}
