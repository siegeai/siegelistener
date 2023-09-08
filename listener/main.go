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
	flag.Parse()

	s, err := newPacketSource(withFileName(*fname), withInterface(*iface))
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
