package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type testTrigger2 struct {
	httpServerMux *http.ServeMux
}

func (t *testTrigger2) Id() string {
	return "testTrigger2"
}

func (t *testTrigger2) Send(ctx context.Context, job chan<- Job) {

	fmt.Println("running trigger 2")
	t.httpServerMux.HandleFunc("/some", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hit")
		if r.Method != http.MethodPost {
			http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		payload := make(map[string]any)
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			fmt.Println("json is fd")
			return
		}

		fmt.Println(payload)

		select {
		case job <- Job{
			Source: t.Id(),
			Payload: payload,
		}:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		case <-ctx.Done():
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		}
	})
}