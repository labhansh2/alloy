package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
)

type Action interface {
	Run(ctx context.Context, payload map[string]any)
}

type Trigger interface {
	Id() string
	Send(ctx context.Context, payload chan<- Job)
}

type Job struct {
	Source  string // this will be the id of the Trigger
	Payload map[string]any
}

type Engine struct {
	HttpClient    *http.Client   // Used for polling/triggers that make outbound HTTP requests
	HttpServerMux *http.ServeMux // Used for incoming webhooks
	Logger        *log.Logger

	triggers   []Trigger
	router     map[string]Action // might need to think about multiple actions later
	jobs       chan Job          // channel buffer size can be variable
	numWorkers int
}

func NewEngine() *Engine {
	return &Engine{
		HttpClient:    &http.Client{},
		HttpServerMux: &http.ServeMux{},
		Logger:        log.New(os.Stdout, "", log.LstdFlags),

		triggers:      make([]Trigger, 0),
		router:        make(map[string]Action),
		jobs:          make(chan Job),
		numWorkers:    runtime.NumCPU(), // we use number of cpus on the device to spawn workers by default
	}
}

func (e *Engine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, t := range e.triggers {
		go t.Send(ctx, e.jobs)
	}

	if e.HttpServerMux != nil {
		go func() {
			if err := http.ListenAndServe(":8080", e.HttpServerMux); err != nil {
				e.Logger.Fatal("Cannot spin up the server")
				panic("Cannot spin up the server")
			}
		}()
	}

	for i := 0; i < e.numWorkers; i++ {
		go e.jobWorker(ctx)
	}
}

func (e *Engine) Shutdown() {
	//might require more cleanups
	e.HttpClient.CloseIdleConnections()
}

func (e *Engine) RegisterFlow(trigger Trigger, action Action) {
	e.triggers = append(e.triggers, trigger)
	e.router[trigger.Id()] = action
}

func (e *Engine) SetNumWorkers(numWorkers int) {
	e.numWorkers = numWorkers
}

func (e *Engine) AddCustomLogger(logger *log.Logger) {
	e.Logger = logger
}

func (e *Engine) jobWorker(ctx context.Context) {
	for {
		select {
		case j := <-e.jobs:
			e.router[j.Source].Run(ctx, j.Payload)
		case <-ctx.Done():
			return
		}
	}
}
