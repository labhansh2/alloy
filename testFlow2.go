package main

import (
	"context"
	"fmt"
	"net/http"
)

type testFlow2 struct {
	httpClient *http.Client
}

func (t *testFlow2) Run(ctx context.Context, payload map[string]any) {
	fmt.Printf("Received %v\n", payload)
	resp, err := t.httpClient.Get("http://localhost:8000/")
	if err != nil {
		fmt.Println("Error fetching from localhost:8000:", err)
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		fmt.Println("Error reading response body:", err)
		return
	}
	fmt.Printf("Response from localhost:8000: %s\n", string(buf[:n]))
}
