package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
)

type Action interface {
	Id() string
	Init(services Services)
	Run(ctx context.Context, payload map[string]any)
}

type Trigger interface {
	Id() string
	Init(services Services)
	Start(ctx context.Context, payload chan<- Job)
}

type Job struct {
	Source  string // this will be the id of the Trigger
	Payload map[string]any
}

// think about adding custom services in the future
type Services struct {
	HttpClient    *http.Client   // Used for polling/triggers that make outbound HTTP requests
	HttpServerMux *http.ServeMux // Used for incoming webhooks
	Logger        *log.Logger
}

type Engine struct {
	services Services // encapsulates all the services like clients, servers and loggers

	triggers   []Trigger
	router     map[string]Action // might need to think about multiple actions later
	jobs       chan Job          // channel buffer size can be variable
	numWorkers int
}

func NewEngine() *Engine {
	return &Engine{

		services: Services{
			HttpClient:    &http.Client{},
			HttpServerMux: &http.ServeMux{},
			Logger:        log.New(os.Stdout, "", log.LstdFlags),
		},

		triggers:   make([]Trigger, 0),
		router:     make(map[string]Action),
		jobs:       make(chan Job),
		numWorkers: runtime.NumCPU(), // we use number of cpus on the device to spawn workers by default
	}
}

func NewEngineWithServices(services Services) *Engine {
	return &Engine{

		services: services,

		triggers:   make([]Trigger, 0),
		router:     make(map[string]Action),
		jobs:       make(chan Job),
		numWorkers: runtime.NumCPU(),
	}
}

func (e *Engine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e.services.Logger.Println("Engine started")

	for _, t := range e.triggers {
		e.services.Logger.Printf("Spinning up trigger: %s\n", t.Id())
		go t.Start(ctx, e.jobs)
	}

	if e.services.HttpServerMux != nil {
		go func() {
			server := &http.Server{
				Addr:    ":8080",
				Handler: e.services.HttpServerMux,
			}
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				e.services.Logger.Fatalf("Cannot spin up the server: %v", err)
			}
		}()
	}

	for i := 0; i < e.numWorkers; i++ {
		go e.jobWorker(ctx)
	}

	// Block forever; exit only on process termination or external context cancellation
	select {}
}

func (e *Engine) Shutdown() {
	//might require more cleanups
	e.services.HttpClient.CloseIdleConnections()
}

func (e *Engine) RegisterFlow(trigger Trigger, action Action) {
	e.services.Logger.Printf("Registering trigger %s to an action", trigger.Id())
	trigger.Init(e.services)
	action.Init(e.services)
	e.triggers = append(e.triggers, trigger)
	e.router[trigger.Id()] = action
}

func (e *Engine) SetNumWorkers(numWorkers int) {
	e.numWorkers = numWorkers
}

func (e *Engine) AddCustomLogger(logger *log.Logger) {
	e.services.Logger = logger
}

func (e *Engine) jobWorker(ctx context.Context) {
	for {
		select {
		case j := <-e.jobs:
			e.router[j.Source].Run(ctx, j.Payload)
		case <-ctx.Done():
			e.services.Logger.Println("Worker stopped")
			return
		}
	}
}
