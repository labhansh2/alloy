package alloy

import (
	"context"
	"errors"
	"log"
	"runtime"
	"strconv"
	"sync"
)

var jobBuff = runtime.NumCPU()

type Payload []byte

type Job struct {
	Source  string
	Payload Payload
}

type Node interface {
	Id() string
	NumInstances() int
	Init(services Services)
	Start(ctx context.Context, workerId string, inJob <-chan Job, outJob chan<- Job)
}

type Dispatcher struct {
	nodes      map[string]Node
	router     map[string][]string
	inChannels map[string]chan Job
	outJobs    chan Job
	logger     *log.Logger
	wg         sync.WaitGroup
}

func NewDispatcher(logger *log.Logger) *Dispatcher {
	return &Dispatcher{
		nodes:      make(map[string]Node),
		router:     make(map[string][]string),
		inChannels: make(map[string]chan Job),
		outJobs:    make(chan Job, jobBuff),
		logger:     logger,
	}
}

func (d *Dispatcher) addNode(node Node) error {
	if _, ok := d.nodes[node.Id()]; ok {
		return errors.New("node already exists with id: " + node.Id())
	}
	d.nodes[node.Id()] = node
	return nil
}

func (d *Dispatcher) addConnection(src, dst string) error {
	if _, ok := d.nodes[src]; !ok {
		return errors.New("node: " + src + "is not registered")
	}
	if _, ok := d.nodes[dst]; !ok {
		return errors.New("node: " + dst + "is not registered")
	}
	d.router[src] = append(d.router[src], dst)
	return nil
}

func (d *Dispatcher) validateGraph() error {
	if len(d.nodes) == 0 {
		return errors.New("no nodes registered")
	}
	for src, dsts := range d.router {
		if _, ok := d.nodes[src]; !ok {
			return errors.New("connection source not found: " + src)
		}
		for _, dst := range dsts {
			if _, ok := d.nodes[dst]; !ok {
				return errors.New("connection destination not found: " + dst)
			}
		}
	}
	return nil
}

func (d *Dispatcher) spinUpNodeWorkers(ctx context.Context) error {
	if err := d.validateGraph(); err != nil {
		return err
	}
	for id, node := range d.nodes {
		ch := make(chan Job, node.NumInstances())
		d.inChannels[id] = ch
		d.logger.Printf("spinning up %d instances of %s", node.NumInstances(), id)
		for i := range node.NumInstances() {
			go node.Start(ctx, node.Id()+strconv.Itoa(i), ch, d.outJobs)
		}
	}
	return nil
}

func (d *Dispatcher) run(ctx context.Context) {
	for i := range jobBuff {
		d.logger.Printf("deploying router %d\n", i)
		d.wg.Go(func() { d.route(ctx, i) })
	}
}

func (d *Dispatcher) route(ctx context.Context, routerId int) {
	for {
		select {
		case <-ctx.Done():
			d.logger.Printf("shutting down router %d\n", routerId)
			return
		case job, ok := <-d.outJobs:
			if !ok {
				d.logger.Printf("shutting donw router %d\n (outJobs chan closed)", routerId)
				return
			}
			for _, dst := range d.router[job.Source] {
				select {
				case <-ctx.Done():
					d.logger.Printf("shutting down router %d\n", routerId)
					return
				case d.inChannels[dst] <- job:
					continue
				}
			}
		}
	}
}

func (d *Dispatcher) shutdown() {
	d.logger.Printf("shutting down dispatcher")
	d.wg.Wait()
	close(d.outJobs)
	for _, ch := range d.inChannels {
		close(ch)
	}
}
