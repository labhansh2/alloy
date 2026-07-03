package alloy

import (
	"context"
	"errors"
	"log"
)

type Payload map[string]any

type Job struct {
	Source  string // this will be the id of the Trigger
	Payload Payload
}

type Node interface {
	Id() string
	NumInstances() int
	Init(services Services)
	Start(ctx context.Context, inJob <-chan Job, outJob chan<- Job)
}

type Dispatcher struct {
	nodes        map[string]Node
	router       map[string][]string
	inChannels   map[string]chan Job
	outJobs      chan Job
	logger *log.Logger
}

func NewDispatcher(logger *log.Logger) *Dispatcher {
	return &Dispatcher{
		nodes:      make(map[string]Node),
		router:     make(map[string][]string),
		inChannels: make(map[string]chan Job),
		outJobs:    make(chan Job, 8),

		logger: logger,
	}
}

func (d *Dispatcher) AddNode(node Node) error {
	if _, ok := d.nodes[node.Id()]; ok{
		return errors.New("Node with this key already exists")
	}
	d.nodes[node.Id()] = node
	return nil
}

func (d *Dispatcher) AddConnection(src string, dst string) {
	if v, ok := d.router[src]; ok {
		v = append(v, dst)
	} else {
		d.router[src] = []string{dst}
	}
}

func (d *Dispatcher) SpinNodeWorkers(ctx context.Context) {

	for id, node := range d.nodes {
		d.logger.Printf("Spinning up %s\n", id)
		d.inChannels[id] = make(chan Job)
		go node.Start(ctx, d.inChannels[id], d.outJobs)
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	for {
		select {
		case job, ok := <-d.outJobs:
			if !ok {
				return
			}
			destinations := d.router[job.Source]
			for i := 0; i < len(destinations); i++ {
				dst := destinations[i]
				select {
				case d.inChannels[dst] <- job:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
