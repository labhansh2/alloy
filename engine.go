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

	// todo: low: custom clients
	// users can use these to have additional global clients
	// can find a better way to add custom clients later
	// i think it should be okay for nodes to initialize their own services in init with the base services
	// however this is an option in case you want a global deposit of custom services
	Clients map[string]*clients.Client
}

type EngineSettings struct {
	tunnelCfg *TunnelCfg
	tunnel    *Tunnel
}

type EngineOptions func(Services, *EngineSettings) error

func WithTunneling(ctx context.Context, cfg *TunnelCfg) EngineOptions {
	return func(s Services, opt *EngineSettings) error {
		t, err := startTunnel(ctx, cfg, s.Logger)
		if err != nil {
			return err
		}
		opt.tunnel = t
		opt.tunnelCfg = cfg
		return nil
	}
}

type Engine struct {
	services   Services
	dispatcher *Dispatcher
	started    bool
	settings   EngineSettings
}

func NewEngine(services Services, opts ...EngineOptions) (*Engine, error) {

	// default mandatory services
	if services.Logger == nil {
		services.Logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}
	if services.HttpClient == nil {
		services.HttpClient = &http.Client{}
	}
	if services.HttpServerMux == nil {
		services.HttpServerMux = &http.ServeMux{}
	}
	if services.httpServer == nil {
		services.httpServer = &http.Server{
			Handler: services.HttpServerMux,
		}
	}

	var settings EngineSettings
	for _, opt := range opts {
		err := opt(services, &settings)
		if err != nil {
			return nil, err
		}
	}

	return &Engine{
		services:   services,
		dispatcher: NewDispatcher(services.Logger),
		started:    false,
		settings:   settings,
	}, nil
}

func (e *Engine) Start(ctx context.Context) error {
	if e.started {
		return errors.New("engine already running")
	}

	e.services.Logger.Println("starting engine")

	if e.settings.tunnel != nil {
		go func() {
			if err := e.services.httpServer.Serve(e.settings.tunnel.listener); err != nil && err != http.ErrServerClosed {
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
	e.started = false
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

func (e *Engine) AddGlobalService(id string, service *clients.Client) error {
	if _, ok := e.services.Clients[id]; ok {
		return errors.New("services with id:" + id + "already exists")
	}
	e.services.Clients[id] = service
	return nil
}

func (e *Engine) PublicURL() string {
	if e.settings.tunnel != nil {
		return e.settings.tunnel.URL()
	}
	return "http://localhost:8080"
}

func (e *Engine) Shutdown() {
	e.log("shutting down engine")
	e.dispatcher.shutdown()
	e.services.HttpClient.CloseIdleConnections()
	e.services.httpServer.Close()
	if e.settings.tunnel != nil {
		e.settings.tunnel.Close()
	}
}

func (e *Engine) log(format string, args ...any) {
	if e.services.Logger != nil {
		e.services.Logger.Printf(format, args...)
	}
}
