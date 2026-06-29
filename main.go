package main

import ()

func main() {

	engine := NewEngine()

	engine.RegisterFlow(&TestTrigger{}, &TestFlow{})
	engine.RegisterFlow(&testTrigger2{}, &TestFlow2{})

	engine.Start()
	// defer engine.Shutdown()
}
