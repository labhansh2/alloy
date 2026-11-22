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
// token authorizes this experiment only; it is not an OpenAI credential.
func New(authPath, nodeToken string) (http.Handler, error) {
	if nodeToken == "" {
		return nil, errors.New("node token is required")
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
	return newHandler(auth.Tokens.AccessToken, auth.Tokens.AccountID, nodeToken, nil), nil
}

func newHandler(accessToken, accountID, nodeToken string, transport http.RoundTripper) http.Handler {
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
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+nodeToken)) != 1 {
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
