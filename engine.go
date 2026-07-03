package alloy

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
)

type Payload map[string]any

type Job struct {
	Source  string // this will be the id of the Trigger
	Payload Payload
}

type Action interface {
	Id() string
	Init(services Services)
	Run(ctx context.Context, payload Payload)
}

type Trigger interface {
	Id() string
	Init(services Services)
	Start(ctx context.Context, job chan<- Job)
}

// think about adding custom services in the future
type Services struct {
	// base
	HttpClient    *http.Client   // Used for polling/triggers that make outbound HTTP requests
	HttpServerMux *http.ServeMux // Used for incoming webhooks
	Logger        *log.Logger

	// // custom
	// AI *clients.AI
	// Notion *clients.Notion
}

type Engine struct {
	services Services // encapsulates all the services like clients, servers and loggers

	triggers   []Trigger
	router     map[string]Action // might need to think about multiple actions later
	jobs       chan Job          // channel buffer size can be variable
	numWorkers int
}

func NewEngine() *Engine {

	c := &http.Client{}

	return &Engine{

		services: Services{

			HttpClient:    c,
			HttpServerMux: &http.ServeMux{},
			Logger:        log.New(os.Stdout, "", log.LstdFlags),
		},

		triggers:   make([]Trigger, 0),
		router:     make(map[string]Action),
		jobs:       make(chan Job),
		numWorkers: runtime.NumCPU(), // we use number of cpus on the device to spawn workers by default
	}
}

func NewEngineWithServices(e *Engine, services Services) *Engine {
	e.services = services
	return e
}

func NewEngineWithBufferedJobs(e *Engine, numBuffer int) *Engine {
	e.jobs = make(chan Job, numBuffer)
	return e
}

func (e *Engine) Start(ctx context.Context) error {
	e.services.Logger.Println("Engine started")

	for _, t := range e.triggers {
		e.services.Logger.Printf("Spinning up trigger: %s\n", t.Id())
		go t.Start(ctx, e.jobs)
	}

	if e.services.HttpServerMux != nil {
		server := &http.Server{
			Addr:    ":8080",
			Handler: e.services.HttpServerMux,
		}

		go func() {
			<-ctx.Done()
			server.Shutdown(context.Background())
		}()

		go func() {
			if err := server.ListenAndServe(); err != nil &&
				err != http.ErrServerClosed {
				e.services.Logger.Printf("server error: %v", err)
			}
		}()
	}

	for i := 0; i < e.numWorkers; i++ {
		go e.jobWorker(i, ctx)
	}

	<-ctx.Done()

	e.Shutdown()
	e.services.Logger.Println("Engine shutting down")
	return nil
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

func (e *Engine) jobWorker(id int, ctx context.Context) {

	e.services.Logger.Printf("Spinning up worker: %d\n", id)

	for {
		select {
		case j, ok := <-e.jobs:
			if !ok {
				return
			}
			action, ok := e.router[j.Source]
			if !ok {
				e.services.Logger.Printf("no action registered for trigger %q", j.Source)
				continue
			}
			e.services.Logger.Printf("")
			action.Run(ctx, j.Payload)
		case <-ctx.Done():
			e.services.Logger.Printf("Worker %d stopped\n", id)
			return
		}
	}
}
