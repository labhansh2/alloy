package alloy

import (
	"context"
	"errors"
	"log"

	"golang.ngrok.com/ngrok/v2"
)

type NgrokTunnel struct {
	Authtoken string
	Domain    string
}

type Tunnel struct {
	listener ngrok.EndpointListener
	url      string
}

func startTunnel(ctx context.Context, cfg *NgrokTunnel, logger *log.Logger) (*Tunnel, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.Authtoken == "" {
		return nil, errors.New("ngrok tunnel requires an authtoken")
	}

	agent, err := ngrok.NewAgent(ngrok.WithAuthtoken(cfg.Authtoken))
	if err != nil {
		return nil, err
	}

	var opts []ngrok.EndpointOption
	if cfg.Domain != "" {
		opts = append(opts, ngrok.WithURL(cfg.Domain))
	}

	ln, err := agent.Listen(ctx, opts...)
	if err != nil {
		return nil, err
	}

	t := &Tunnel{listener: ln, url: ln.URL().String()}
	logger.Printf("ngrok tunnel online: %s", t.url)
	return t, nil
}

func (t *Tunnel) URL() string {
	if t == nil {
		return ""
	}
	return t.url
}

func (t *Tunnel) Close() error {
	if t == nil || t.listener == nil {
		return nil
	}
	return t.listener.Close()
}
