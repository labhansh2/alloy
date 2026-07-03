package alloy

import (
	"context"
	"log"
	"net/http"
	"os"
)

// think about adding custom services in the future
type Services struct {
	HttpClient    *http.Client   // Used for polling/triggers that make outbound HTTP requests
	HttpServerMux *http.ServeMux // Used for incoming webhooks
	Logger        *log.Logger
}

type Engine struct {
	services   Services // encapsulates all the services like clients, servers and loggers
	dispatcher Dispatcher
}

func NewEngine() *Engine {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	return &Engine{
		services: Services{

			HttpClient:    &http.Client{},
			HttpServerMux: &http.ServeMux{},
			Logger:        logger,
		},
		dispatcher: *NewDispatcher(logger),
	}
}

func NewEngineWithServices(e *Engine, services Services) *Engine {
	e.services = services
	return e
}

func (e *Engine) Start(ctx context.Context) error {
	e.services.Logger.Println("Starting Engine")
	e.dispatcher.SpinNodeWorkers(ctx)

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

	e.dispatcher.Start(ctx)
	<-ctx.Done()

	e.Shutdown()
	e.services.Logger.Println("Engine shutting down")
	return nil
}

func (e *Engine) RegisterNode(node Node) error {
	node.Init(e.services)
	err := e.dispatcher.AddNode(node)
	return err
}

func (e *Engine) RegisterNodes(nodes []Node) error {
	for _, n := range nodes {
		err := e.RegisterNode(n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) RegisterConnection(source string, destination string) {
	e.dispatcher.AddConnection(source, destination)
}

func (e *Engine) RegisterConnections(connections map[string][]string) {
	for src, dsts := range connections {
		for _, dst := range dsts {
			e.RegisterConnection(src, dst)
		}
	}
}

func (e *Engine) AddCustomLogger(logger *log.Logger) {
	e.services.Logger = logger
}

func (e *Engine) Shutdown() {
	//might require more cleanups
	e.services.HttpClient.CloseIdleConnections()
}