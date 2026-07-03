package alloy

import (
	"context"
	"io"
	"net/http"
)

type Webhook struct {
	C chan []byte
}

func NewWebhook(ctx context.Context, httpMux *http.ServeMux, url string) *Webhook {
	w := Webhook{
		C: make(chan []byte),
	}
	w.listen(ctx, httpMux, url)

	return &w
}

func (wh *Webhook) listen(ctx context.Context, httpMux *http.ServeMux, url string) {

	httpMux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

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

		select {
		case wh.C <- data:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		case <-ctx.Done():
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		}
	})
}
