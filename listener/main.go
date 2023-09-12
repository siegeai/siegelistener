package main

import (
	"flag"
	"log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	iface := flag.String("i", "eth0", "Interface to get packets from")
	fname := flag.String("r", "", "Filename to read from, overrides -i")
	port := flag.Int("p", 80, "Port to listen on")
	flag.Parse()

	s, err := newPacketSource(withFileName(*fname), withInterface(*iface), withPort(*port))
	if err != nil {
		return err
	}

	l := newListener(s)

	err = l.run()
	if err != nil {
		return err
	}

	return nil
}
