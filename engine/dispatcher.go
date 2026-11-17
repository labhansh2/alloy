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
	// Type identifies the event for subscription routing. Source is provenance.
	Type    string
	Source  string
	Id      string
	Payload Payload
}

type Node interface {
	Id() string
	NumInstances() int
	Init(services Services) error
	Start(ctx context.Context, workerId string, inJob <-chan Job, outJob chan<- Job)
}

type Dispatcher struct {
	nodes         map[string]Node
	router        map[string][]string
	subscriptions map[string][]string
	workers       sync.WaitGroup
	inChannels    map[string]chan Job
	outJobs       chan Job
	logger        *log.Logger
	wg            sync.WaitGroup
}

func NewDispatcher(logger *log.Logger) *Dispatcher {
	return &Dispatcher{
		nodes:         make(map[string]Node),
		subscriptions: make(map[string][]string),
		router:        make(map[string][]string),
		inChannels:    make(map[string]chan Job),
		outJobs:       make(chan Job, jobBuff),
		logger:        logger,
	}
}

func (d *Dispatcher) addNode(node Node) error {
	if node.Id() == "" || node.NumInstances() < 1 {
		return errors.New("node requires an ID and at least one worker")
	}
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
			d.workers.Go(func() { node.Start(ctx, node.Id()+strconv.Itoa(i), ch, d.outJobs) })
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
			for _, dst := range d.destinations(job) {
				delivery := job
				delivery.Payload = append(Payload(nil), job.Payload...)
				select {
				case <-ctx.Done():
					d.logger.Printf("shutting down router %d\n", routerId)
					return
				case d.inChannels[dst] <- delivery:
					continue
				}
			}
		}
	}
}

func (d *Dispatcher) shutdown() {
	d.logger.Printf("shutting down dispatcher")
	d.wg.Wait()
	d.workers.Wait()
	close(d.outJobs)
	for _, ch := range d.inChannels {
		close(ch)
	}
}

// subscribe registers once per logical node; its workers share deliveries.
func (d *Dispatcher) subscribe(eventType, nodeID string) error {
	if eventType == "" {
		return errors.New("event type is required")
	}
	if _, ok := d.nodes[nodeID]; !ok {
		return errors.New("unknown subscriber: " + nodeID)
	}
	for _, id := range d.subscriptions[eventType] {
		if id == nodeID {
			return nil
		}
	}
	d.subscriptions[eventType] = append(d.subscriptions[eventType], nodeID)
	return nil
}

func (d *Dispatcher) destinations(job Job) []string {
	if job.Type != "" {
		return d.subscriptions[job.Type]
	}
	// Legacy source routing applies only to untyped jobs during migration.
	return d.router[job.Source]
}
