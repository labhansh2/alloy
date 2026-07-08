package notion

import (
	"errors"
	"log"
	"net/http"
)

func RequestVerifier(secret string, logger *log.Logger) func(r *http.Request, body []byte) error {
	if secret == "" && logger != nil {
		logger.Println("webhook needs to be verified — set NOTION_VERIFICATION_TOKEN after provider handshake")
	}

	return func(r *http.Request, body []byte) error {
		signature := r.Header.Get("X-Notion-Signature")
		token := secret
		if token == "" {
			handshakeToken, ok := VerificationTokenFromBody(body)
			if !ok {
				return errors.New("invalid signature")
			}
			token = handshakeToken
		}

		if !VerifyWebhookSignature(body, signature, token) {
			return errors.New("invalid signature")
		}

		if handshakeToken, ok := VerificationTokenFromBody(body); ok && logger != nil {
			logger.Printf("notion webhook verification handshake — paste this token in Notion dashboard: %s", handshakeToken)
		}

		return nil
	}
}
