package openai

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"alloy/provider"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const planScopes = "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"

func (m *manager) login(ctx context.Context, options provider.LoginOptions) error {
	if options.Method != "" && options.Method != "browser" && options.Method != "token" {
		return errors.New("OpenAI login method must be browser or token")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := m.ensureStore(); err != nil {
		return err
	}

	return m.store.locked(ctx, func(record credentialRecord) error {
		if options.Method == "token" {
			previous, err := record.load()
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return errors.New("existing OpenAI credentials cannot be read; refusing to overwrite them")
			}

			if previous != nil && previous.Mode == "browser" {
				return errors.New("this account uses ChatGPT login; use another WithAccount label for an API key")
			}

			key, err := m.secret()
			if err != nil {
				return err
			}

			if _, err := provider.BearerToken(key); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if err := record.save(&credential{Mode: "token", Token: &oauth2.Token{AccessToken: key, TokenType: "Bearer"}}); err != nil {
				return err
			}

			m.output("OpenAI API key saved securely. API usage is billed separately from ChatGPT plans.")

			return nil
		}

		return m.browserLogin(ctx, record)
	})
}

type callback struct {
	code, clientID string
}

func (m *manager) browserLogin(ctx context.Context, record credentialRecord) error {
	previous, err := record.load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("existing OpenAI credentials could not be read; refusing to overwrite them")
	}

	hostID, err := record.hostID()
	if err != nil {
		return err
	}

	clientID := "dynamic_agent_client"
	if previous != nil && previous.Mode == "browser" {
		clientID = previous.ClientID
	}

	state, nonce, verifier := randomValue(), randomValue(), oauth2.GenerateVerifier()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("cannot start OpenAI login callback")
	}
	defer listener.Close()

	redirect := "http://" + listener.Addr().String() + "/auth/callback"
	config := oauth2.Config{
		ClientID: clientID, RedirectURL: redirect, Endpoint: m.endpoint,
		Scopes: strings.Fields(planScopes),
	}

	options := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("resource", apiURL), oauth2.SetAuthURLParam("ext_agent_host_id", hostID),
	}
	if clientID == "dynamic_agent_client" {
		options = append(options, oauth2.SetAuthURLParam("agent_name_hint", "Alloy"))
	}

	// Do not put retained ID tokens in displayed/logged authorization URLs.
	authorizeURL := config.AuthCodeURL(state, options...)
	result := make(chan callback, 1)
	failed := make(chan struct{}, 1)

	var (
		consumed bool
		mu       sync.Mutex
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")

		query := r.URL.Query()
		if len(query["state"]) != 1 || subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
			http.Error(w, "invalid login state", http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		if consumed {
			http.Error(w, "login callback already used", http.StatusBadRequest)
			return
		}

		consumed = true

		issued := query.Get("client_id")
		if query.Get("error") != "" || len(query["client_id"]) > 1 || len(query["code"]) != 1 || query.Get("code") == "" ||
			(clientID == "dynamic_agent_client" && (issued == "" || issued == "dynamic_agent_client")) ||
			(clientID != "dynamic_agent_client" && issued != "" && issued != clientID) {
			failed <- struct{}{}

			http.Error(w, "login was not authorized", http.StatusBadRequest)

			return
		}

		if issued == "" {
			issued = clientID
		}

		result <- callback{code: query.Get("code"), clientID: issued}

		_, _ = io.WriteString(w, "Authorization received. Return to the terminal to finish verification.")
	})

	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)

	go func() { done <- httpServer.Serve(listener) }()

	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = httpServer.Shutdown(shutdown)
		_ = httpServer.Close()

		<-done
	}()

	m.output("Continue with ChatGPT to authorize eligible plan usage for this local Alloy account. This is separate from API-key billing.")
	m.output("Authorize in your browser: " + authorizeURL)

	if err := m.open(ctx, authorizeURL); err != nil {
		m.output("Could not open a browser automatically; open the authorization link above on this machine.")
	}

	var authorized callback
	select {
	case authorized = <-result:
	case <-failed:
		return errors.New("OpenAI authorization declined or invalid; no credentials changed")
	case <-ctx.Done():
		return errors.New("OpenAI login canceled or timed out")
	case err := <-done:
		done <- err
		return errors.New("OpenAI login callback stopped")
	}

	config.ClientID = authorized.clientID
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, m.client)

	token, err := config.Exchange(tokenCtx, authorized.code, oauth2.VerifierOption(verifier), oauth2.SetAuthURLParam("resource", apiURL))
	if err != nil {
		return errors.New("OpenAI code exchange failed; retry the server's explicit login flow")
	}

	idToken, _ := token.Extra("id_token").(string)
	scopes, _ := token.Extra("scope").(string)

	subject, email, err := m.verify(ctx, idToken, authorized.clientID, nonce, token.AccessToken)
	if err != nil || !hasPlanScopes(scopes) || token.AccessToken == "" || token.RefreshToken == "" || !token.Expiry.After(time.Now()) {
		return errors.New("OpenAI identity, expiry, or plan-usage scopes could not be verified; no credentials changed")
	}

	if previous != nil && previous.Mode == "browser" && previous.Subject != subject {
		return errors.New("ChatGPT account changed; choose a different WithAccount label instead of overwriting this account")
	}

	saved := &credential{Mode: "browser", ClientID: authorized.clientID, Subject: subject, Email: email, IDToken: idToken, Scopes: scopes, Token: token}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := record.save(saved); err != nil {
		return err
	}

	m.output("ChatGPT authorization saved securely for account " + m.account + ". Review app access and limits at https://chatgpt.com/settings/usage")

	return nil
}

func randomValue() string {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		panic("cryptographic random source unavailable")
	}

	return base64.RawURLEncoding.EncodeToString(value)
}

func (m *manager) verify(ctx context.Context, raw, clientID, nonce, accessToken string) (string, string, error) {
	ctx = oidc.ClientContext(ctx, m.client)

	issuer, err := oidc.NewProvider(ctx, m.issuer)
	if err != nil {
		return "", "", errors.New("OpenAI identity verification unavailable")
	}

	verified, err := issuer.Verifier(&oidc.Config{ClientID: clientID, SupportedSigningAlgs: []string{"RS256", "ES256"}}).Verify(ctx, raw)
	if err != nil || verified.Subject == "" || (nonce != "" && subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(nonce)) != 1) {
		return "", "", errors.New("OpenAI identity verification failed")
	}

	if verified.IssuedAt.IsZero() || verified.IssuedAt.After(time.Now().Add(time.Minute)) ||
		(verified.AccessTokenHash != "" && verified.VerifyAccessToken(accessToken) != nil) {
		return "", "", errors.New("OpenAI token time or access-token binding is invalid")
	}

	var claims struct {
		Email           string `json:"email"`
		AuthorizedParty string `json:"azp"`
	}
	if err := verified.Claims(&claims); err != nil {
		return "", "", errors.New("invalid OpenAI identity claims")
	}

	if (claims.AuthorizedParty != "" && claims.AuthorizedParty != clientID) || (len(verified.Audience) > 1 && claims.AuthorizedParty == "") {
		return "", "", errors.New("invalid OpenAI authorized party")
	}

	return verified.Subject, claims.Email, nil
}

func (m *manager) refresh(ctx context.Context, record credentialRecord, saved *credential) error {
	form := url.Values{
		"grant_type": {"refresh_token"}, "client_id": {saved.ClientID},
		"refresh_token": {saved.Token.RefreshToken}, "resource": {apiURL},
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return errLoginRequired
	}

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := m.client.Do(r)
	if err != nil {
		return errors.New("OpenAI token refresh unavailable; retry or reauthenticate on the server")
	}
	defer response.Body.Close()

	var renewed struct {
		AccessToken  string  `json:"access_token"`
		RefreshToken string  `json:"refresh_token"`
		IDToken      string  `json:"id_token"`
		TokenType    string  `json:"token_type"`
		ExpiresIn    int64   `json:"expires_in"`
		Scope        *string `json:"scope"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&renewed) != nil ||
		renewed.AccessToken == "" || renewed.ExpiresIn <= 0 || renewed.ExpiresIn > 365*24*60*60 || !strings.EqualFold(renewed.TokenType, "Bearer") {
		return errors.New("OpenAI token refresh failed; reauthenticate on the server")
	}

	next := *saved

	next.Token = &oauth2.Token{AccessToken: renewed.AccessToken, RefreshToken: renewed.RefreshToken, TokenType: "Bearer", Expiry: time.Now().Add(time.Duration(renewed.ExpiresIn) * time.Second)}
	if next.Token.RefreshToken == "" {
		next.Token.RefreshToken = saved.Token.RefreshToken
	}

	if renewed.Scope != nil {
		next.Scopes = *renewed.Scope
	}

	if renewed.IDToken != "" {
		subject, email, err := m.verify(ctx, renewed.IDToken, saved.ClientID, "", renewed.AccessToken)
		if err != nil || subject != saved.Subject {
			return errors.New("OpenAI refresh changed account identity; reauthenticate on the server")
		}

		next.IDToken, next.Email = renewed.IDToken, email
	}

	if err := record.save(&next); err != nil {
		return err
	}

	if !hasPlanScopes(next.Scopes) {
		return errLoginRequired
	}

	*saved = next

	return nil
}
