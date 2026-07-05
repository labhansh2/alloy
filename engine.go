package alloy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
)

type Services struct {
	HttpClient    *http.Client
	httpServer    *http.Server
	HttpServerMux *http.ServeMux
	Logger        *log.Logger
}

type Engine struct {
	services   Services
	dispatcher *Dispatcher
	started    bool
}

func NewEngine() *Engine {
	return NewEngineWithServices(Services{})
}

func NewEngineWithServices(services Services) *Engine {
	if services.Logger == nil {
		services.Logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}
	if services.HttpClient == nil {
		services.HttpClient = &http.Client{}
	}
	if services.HttpServerMux == nil {
		services.HttpServerMux = &http.ServeMux{}
	}
	return &Engine{
		services:   services,
		dispatcher: NewDispatcher(services.Logger),
	}
}

func (e *Engine) Start(ctx context.Context) error {
	if e.started {
		return errors.New("engine already running")
	}

	e.services.Logger.Println("starting engine")

	if e.services.HttpServerMux == nil {
		return errors.New("HttpServerMux is missing in services")
	}

	e.services.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: e.services.HttpServerMux,
	}

	go func() {
		if err := e.services.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			e.services.Logger.Printf("server error: %v", err)
		}
	}()

	e.dispatcher.spinUpNodeWorkers(ctx)
	e.dispatcher.run(ctx)
	e.started = true

	<- ctx.Done()
	return nil
}

func (e *Engine) RegisterNode(node Node) error {
	if e.started {
		return errors.New("cannot register node on a running engine")
	}
	node.Init(e.services)
	return e.dispatcher.addNode(node)
}

func (e *Engine) RegisterNodes(nodes []Node) error {
	for _, n := range nodes {
		if err := e.RegisterNode(n); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) RegisterConnection(source, destination string) error {
	if e.started {
		return errors.New("cannot register connection on a running eninge")
	}
	return e.dispatcher.addConnection(source, destination)
}

func (e *Engine) RegisterConnections(connections map[string][]string) error {
	for src, dsts := range connections {
		for _, dst := range dsts {
			if err := e.RegisterConnection(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) Shutdown() {
	e.log("shutting down engine")
	e.dispatcher.shutdown()
	e.services.HttpClient.CloseIdleConnections()
	e.services.httpServer.Close()
}

func (e *Engine) log(format string, args ...any) {
	if e.services.Logger != nil {
		e.services.Logger.Printf(format, args...)
	}
}
