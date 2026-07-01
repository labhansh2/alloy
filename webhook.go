package alloy

import (
	"context"
	"encoding/json"
	"net/http"
)

type Webhook struct {
	C chan map[string]any
}

func NewWebhook(ctx context.Context, httpMux *http.ServeMux, url string) *Webhook {

	w := Webhook{
		C: make(chan map[string]any),
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

		data := make(map[string]any)
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
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
