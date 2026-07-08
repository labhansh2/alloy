package alloy

import (
	"alloy/clients"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
)

type Services struct {
	// base servies
	// these cannot be nil
	HttpClient    *http.Client
	httpServer    *http.Server
	HttpServerMux *http.ServeMux
	Logger        *log.Logger
	Tunnel        *NgrokTunnel

	// custom clients
	// users can use these to have additional global clients
	// can find a better way to add custom clients later
	Notion *clients.Client
	AI     *clients.Client
}

type Engine struct {
	services   Services
	dispatcher *Dispatcher
	tunnel     *Tunnel
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

	tunnel, err := startTunnel(ctx, e.services.Tunnel, e.services.Logger)
	if err != nil {
		return err
	}
	e.tunnel = tunnel

	e.services.httpServer = &http.Server{
		Handler: e.services.HttpServerMux,
	}

	if tunnel != nil {
		go func() {
			if err := e.services.httpServer.Serve(tunnel.listener); err != nil && err != http.ErrServerClosed {
				e.services.Logger.Printf("server error: %v", err)
			}
		}()
	} else {
		e.services.httpServer.Addr = ":8080"
		go func() {
			if err := e.services.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				e.services.Logger.Printf("server error: %v", err)
			}
		}()
	}

	e.services.Logger.Printf("http server listening at %s", e.PublicURL())

	e.dispatcher.spinUpNodeWorkers(ctx)
	e.dispatcher.run(ctx)
	e.started = true

	<-ctx.Done()
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

func (e *Engine) PublicURL() string {
	if e.tunnel != nil {
		return e.tunnel.URL()
	}
	return "http://localhost:8080"
}

func (e *Engine) Shutdown() {
	e.log("shutting down engine")
	e.dispatcher.shutdown()
	e.services.HttpClient.CloseIdleConnections()
	e.services.httpServer.Close()
	if e.tunnel != nil {
		e.tunnel.Close()
	}
}

func (e *Engine) log(format string, args ...any) {
	if e.services.Logger != nil {
		e.services.Logger.Printf(format, args...)
	}
}
