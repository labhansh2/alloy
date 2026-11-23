// Package codexproxy implements an experimental, loopback-only Codex auth
// broker. It does not implement login, refresh, or remote deployment.
package codexproxy

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// New reads the host login without returning its secrets to callers. The node
// tokens authorize this experiment only; they are not OpenAI credentials.
func New(authPath string, nodeTokens ...string) (http.Handler, error) {
	if len(nodeTokens) == 0 {
		return nil, errors.New("node token is required")
	}

	for _, token := range nodeTokens {
		if token == "" {
			return nil, errors.New("node tokens must not be empty")
		}
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		return nil, errors.New("cannot read host Codex auth file")
	}

	var auth struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
			AccountID   string `json:"account_id"`
		} `json:"tokens"`
	}
	if json.Unmarshal(data, &auth) != nil || auth.Tokens.AccessToken == "" {
		return nil, errors.New("host Codex auth file does not contain a ChatGPT access token")
	}

	return newHandler(auth.Tokens.AccessToken, auth.Tokens.AccountID, nodeTokens, nil), nil
}

func newHandler(accessToken, accountID string, nodeTokens []string, transport http.RoundTripper) http.Handler {
	nodeTokens = append([]string(nil), nodeTokens...)
	target, _ := url.Parse("https://chatgpt.com")
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.URL.Path = "/backend-api/codex/responses"
			r.Out.URL.RawQuery = ""
			r.Out.Header.Set("Authorization", "Bearer "+accessToken)
			r.Out.Header.Del("ChatGPT-Account-ID")

			if accountID != "" {
				r.Out.Header.Set("ChatGPT-Account-ID", accountID)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Codex upstream unavailable", http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched := 0
		for _, token := range nodeTokens {
			matched |= subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token))
		}

		if matched != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method != http.MethodPost || r.URL.Path != "/codex/responses" {
			http.NotFound(w, r)
			return
		}

		proxy.ServeHTTP(w, r)
	})
}
