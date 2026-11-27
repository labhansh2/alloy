// Package openai supplies OpenAI authentication and native SDK configuration.
// It does not wrap the inference API or maintain a model catalog.
package openai

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"alloy/provider"

	native "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"golang.org/x/oauth2"
)

const apiURL = "https://api.openai.com/v1"

type settings struct{ account string }

type Option func(*settings)

// WithAccount selects a separate, named local account registration. Re-login
// cannot silently replace its verified ChatGPT identity with another account.
func WithAccount(name string) Option {
	return func(s *settings) { s.account = name }
}

// Provider registers managed OpenAI auth. Saved account credentials take
// precedence; OPENAI_API_KEY is used only when this profile has no saved login.
// No browser is opened and no credentials are read until CheckAuth/Login/Auth.
func Provider(options ...Option) provider.Config {
	s := settings{account: "default"}
	for _, option := range options {
		option(&s)
	}

	m := newManager(s.account)

	return provider.Config{BaseURL: apiURL, Auth: m.authenticate, CheckAuth: m.check, Login: m.login}
}

// Client returns the native OpenAI SDK configured for an Alloy connection.
// No process, background worker, or second node session is required.
func Client(connection provider.Connection) native.Client {
	return native.NewClient(
		option.WithBaseURL(connection.BaseURL()),
		option.WithAPIKey(connection.Token()),
		option.WithHTTPClient(connection.HTTPClient()),
		option.WithMaxRetries(0),
		option.WithOrganization(""), option.WithProject(""),
	)
}

var errLoginRequired = errors.New("OpenAI authentication required; run with --login to continue with ChatGPT, or --login --auth token to save an API key")

type credential struct {
	Mode     string        `json:"mode"`
	ClientID string        `json:"client_id,omitempty"`
	Subject  string        `json:"subject,omitempty"`
	Email    string        `json:"email,omitempty"`
	IDToken  string        `json:"id_token,omitempty"`
	Scopes   string        `json:"scopes,omitempty"`
	Token    *oauth2.Token `json:"token"`
}

type credentialStore interface {
	locked(context.Context, func() error) error
	load() (*credential, error)
	save(*credential) error
	hostID() (string, error)
}

type manager struct {
	mu       sync.Mutex
	store    credentialStore
	account  string
	issuer   string
	endpoint oauth2.Endpoint
	client   *http.Client
	open     func(context.Context, string) error
	output   func(string)
	secret   func() (string, error)
	getenv   func(string) string
}

func newManager(account string) *manager {
	return &manager{
		account: account,
		issuer:  "https://auth.openai.com",
		endpoint: oauth2.Endpoint{
			AuthURL:   "https://auth.openai.com/api/accounts/authorize",
			TokenURL:  "https://auth.openai.com/api/accounts/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		client: &http.Client{Transport: &http.Transport{Proxy: nil, IdleConnTimeout: 30 * time.Second}, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("authentication redirects are not supported")
		}},
		open: openBrowser, output: printLogin, secret: readAPIKey, getenv: os.Getenv,
	}
}

func (m *manager) ensureStore() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store != nil {
		return nil
	}

	store, err := newStore(m.account)
	if err != nil {
		return err
	}

	m.store = store

	return nil
}

func (m *manager) check(ctx context.Context) error {
	_, err := m.accessToken(ctx)
	return err
}

func (m *manager) authenticate(r *http.Request) error {
	token, err := m.accessToken(r.Context())
	if err != nil {
		return err
	}

	r.Header.Set("Authorization", "Bearer "+token)

	return nil
}

func (m *manager) accessToken(ctx context.Context) (string, error) {
	if err := m.ensureStore(); err != nil {
		return "", err
	}

	var result string

	err := m.store.locked(ctx, func() error {
		saved, err := m.store.load()
		if errors.Is(err, os.ErrNotExist) {
			key := m.getenv("OPENAI_API_KEY")
			if _, err := provider.BearerToken(key); err != nil {
				return errLoginRequired
			}

			result = key

			return nil
		}

		if err != nil {
			return err
		}

		if saved.Token == nil {
			return errLoginRequired
		}

		if saved.Mode == "token" {
			if _, err := provider.BearerToken(saved.Token.AccessToken); err != nil {
				return errLoginRequired
			}
		} else if saved.Mode == "browser" {
			if !hasPlanScopes(saved.Scopes) || saved.ClientID == "" || saved.Subject == "" {
				return errLoginRequired
			}

			if saved.Token.AccessToken == "" || !saved.Token.Expiry.After(time.Now().Add(time.Minute)) {
				if err := m.refresh(ctx, saved); err != nil {
					return err
				}
			}
		} else {
			return errLoginRequired
		}

		result = saved.Token.AccessToken

		return nil
	})

	return result, err
}

func hasPlanScopes(scopes string) bool {
	set := make(map[string]bool)
	for _, scope := range strings.Fields(scopes) {
		set[scope] = true
	}

	return set["chatgpt.tokens.use.direct"] && set["resource.invoke"]
}
