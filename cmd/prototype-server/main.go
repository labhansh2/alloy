// Command prototype-server runs the local process-node communication proof.
package main

import (
	"log"
	"net/http"

	"alloy/internal/prototype/server"
)

func main() {
	addr := server.ListenAddress()
	log.Printf("prototype server listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server.NewHandler(log.Default())))
}
