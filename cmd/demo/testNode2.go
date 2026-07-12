package main

import (
	"alloy"
	"context"
	"encoding/json"
	"log"
	"net/http"
)

type TestNode2 struct {
	httpClient *http.Client
	logger     *log.Logger
}

func (t *TestNode2) Id() string { return "TestNode2" }

func (t *TestNode2) NumInstances() int { return 1 }

func (t *TestNode2) Init(services alloy.Services) error {
	t.httpClient = services.HttpClient
	t.logger = services.Logger
	return nil
}

func (t *TestNode2) Start(ctx context.Context, workerId string, inJob <-chan alloy.Job, _ chan<- alloy.Job) {

	type random struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	for {
		select {
		case <-ctx.Done():
			t.logger.Printf("shutting down worker %s\n", workerId)
			return
		case j, ok := <-inJob:
			if !ok {
				t.logger.Printf("shutting down worker: %s (job chan closed)", workerId)
				return
			}

			var r random
			e := json.Unmarshal(j.Payload, &r)
			if e != nil {
				t.logger.Println("Bruh")
			}
			t.logger.Printf("Recieved payload %v", r)
			// resp, err := t.httpClient.Get("http://localhost:8000/")
			// if err != nil {
			// 	t.logger.Printf("[%s] %s: error fetching from localhost:8000: %v", workerId, t.Id(), err)
			// 	return
			// }
			// func() {
			// 	defer resp.Body.Close()
			// 	buf := make([]byte, 1024)
			// 	n, err := resp.Body.Read(buf)
			// 	if err != nil && err != io.EOF {
			// 		t.logger.Printf("[%s] %s: error reading response body: %v", workerId, t.Id(), err)
			// 		return
			// 	}
			// 	t.logger.Printf("[%s] %s: response from localhost:8000: %s", workerId, t.Id(), string(buf[:n]))
			// }()
		}
	}
}
