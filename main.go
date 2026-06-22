package main

import ()

func main() {

	engine := NewEngine()

	engine.RegisterFlow(&testTrigger{}, &testFlow{})
	engine.RegisterFlow(&testTrigger2{engine.HttpServerMux}, &testFlow2{engine.HttpClient})

	engine.Start()
	defer engine.Shutdown()
}
