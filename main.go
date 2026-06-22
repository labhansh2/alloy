package main

import (
	"net/http"
)

func main() {

	engine := Engine{
		HttpClient: &http.Client{},
		HttpServerMux: http.NewServeMux(),
		Triggers: make([]Trigger, 0),
		Router: make(map[string]Flow),
		Jobs: make(chan Job, 5),
	}

	tt := testTrigger{}
	fl := testFlow{}

	tt2 := testTrigger2{engine.HttpServerMux}
	fl2 := testFlow2{engine.HttpClient}

	// encapsulate this logic
	engine.Triggers = append(engine.Triggers, &tt)
	engine.Router[tt.Id()] = &fl

	engine.Triggers = append(engine.Triggers, &tt2)
	engine.Router[tt2.Id()] = &fl2

	engine.Start()
}