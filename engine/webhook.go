package alloy

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
)

var ErrWebhookUnverified = errors.New("webhook needs to be verified")

type WebhookVerifyFunc func(r *http.Request, body []byte) error

type Webhook struct {
	C chan []byte
}

type webhookSettings struct {
	bufferSize int
	verify     WebhookVerifyFunc
	needsAuth  bool
	logger     *log.Logger
	path       string
}

type WebhookOption func(*webhookSettings)

func WithWebhookBuffer(bufferSize int) WebhookOption {
	return func(s *webhookSettings) {
		s.bufferSize = bufferSize
	}
}

func WithWebhookVerify(verify WebhookVerifyFunc) WebhookOption {
	return func(s *webhookSettings) {
		s.verify = verify
	}
}

// RequiresAuth marks a webhook as needing signature verification. If no verifier
// is configured, a reminder is logged at registration time.
func RequiresAuth() WebhookOption {
	return func(s *webhookSettings) {
		s.needsAuth = true
	}
}

func WithWebhookLogger(logger *log.Logger) WebhookOption {
	return func(s *webhookSettings) {
		s.logger = logger
	}
}

func NewWebhook(
	ctx context.Context,
	httpMux *http.ServeMux,
	path string,
	opts ...WebhookOption,
) *Webhook {
	settings := webhookSettings{path: path}
	for _, opt := range opts {
		opt(&settings)
	}

	if settings.needsAuth && settings.verify == nil && settings.logger != nil {
		settings.logger.Printf("webhook %s needs to be verified", path)
	}

	var ch chan []byte
	if settings.bufferSize > 0 {
		ch = make(chan []byte, settings.bufferSize)
	} else {
		ch = make(chan []byte)
	}

	w := Webhook{C: ch}
	w.listen(ctx, httpMux, settings)
	return &w
}

func (wh *Webhook) listen(
	ctx context.Context,
	httpMux *http.ServeMux,
	settings webhookSettings,
) {
	httpMux.HandleFunc(settings.path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if settings.verify != nil {
			if err := settings.verify(r, data); err != nil {
				if errors.Is(err, ErrWebhookUnverified) {
					http.Error(w, "webhook needs to be verified", http.StatusUnauthorized)
					return
				}
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		select {
		case wh.C <- data:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		case <-ctx.Done():
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		}
	})
}
