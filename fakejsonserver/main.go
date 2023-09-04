package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	host := flag.String("h", "", "the host to listen on")
	port := flag.String("p", "80", "the port to listen on")
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Println("Listening at", addr)

	s := newServer()
	s.populateTestWidgets()
	s.setupRoutes()

	return http.ListenAndServe(addr, s.router)
}
