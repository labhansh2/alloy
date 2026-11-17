package alloy

import (
	"context"
	"io"
	"log"
	"testing"
	"time"
)

type subscriptionNode struct {
	id       string
	received chan Job
	stopped  chan struct{}
}

func (n *subscriptionNode) Id() string          { return n.id }
func (n *subscriptionNode) NumInstances() int   { return 1 }
func (n *subscriptionNode) Init(Services) error { return nil }
func (n *subscriptionNode) Start(ctx context.Context, _ string, in <-chan Job, _ chan<- Job) {
	defer close(n.stopped)
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-in:
			if !ok {
				return
			}
			select {
			case n.received <- j:
			case <-ctx.Done():
				return
			}
		}
	}
}
func TestSubscriptionFanoutAndShutdown(t *testing.T) {
	d := NewDispatcher(log.New(io.Discard, "", 0))
	nodes := []*subscriptionNode{
		{"a", make(chan Job, 1), make(chan struct{})},
		{"b", make(chan Job, 1), make(chan struct{})},
	}
	for _, n := range nodes {
		if err := d.addNode(n); err != nil {
			t.Fatal(err)
		}
		if err := d.subscribe("changed", n.id); err != nil {
			t.Fatal(err)
		}
		if err := d.subscribe("changed", n.id); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.destinations(Job{Type: "changed"})) != 2 {
		t.Fatal("duplicate subscriptions")
	}
	if err := d.subscribe("changed", "missing"); err == nil {
		t.Fatal("accepted unknown node")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.spinUpNodeWorkers(ctx); err != nil {
		t.Fatal(err)
	}
	d.run(ctx)
	defer d.shutdown()
	defer cancel()
	d.outJobs <- Job{Type: "changed", Source: "unregistered-producer", Payload: Payload("hello")}
	var got []Job
	for _, n := range nodes {
		select {
		case j := <-n.received:
			got = append(got, j)
		case <-time.After(time.Second):
			t.Fatal("delivery missing")
		}
	}
	got[0].Payload[0] = 'X'
	if string(got[1].Payload) != "hello" {
		t.Fatal("subscribers share mutable payload")
	}
	cancel()
	for _, n := range nodes {
		select {
		case <-n.stopped:
		case <-time.After(time.Second):
			t.Fatal("worker did not stop")
		}
	}
}
