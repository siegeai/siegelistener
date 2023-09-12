package main

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"log"
)

type newPacketSourceArgs struct {
	f string // filename
	i string // interface
	p int
}

type newPacketSourceOpt func(*newPacketSourceArgs)

func withFileName(f string) newPacketSourceOpt {
	return func(args *newPacketSourceArgs) {
		args.f = f
	}
}

func withInterface(i string) newPacketSourceOpt {
	return func(args *newPacketSourceArgs) {
		args.i = i
	}
}

func withPort(p int) newPacketSourceOpt {
	return func(args *newPacketSourceArgs) {
		args.p = p
	}
}

func newPacketSource(opts ...newPacketSourceOpt) (*gopacket.PacketSource, error) {
	args := newPacketSourceArgs{f: "", i: "lo", p: 80}
	for _, opt := range opts {
		opt(&args)
	}

	var handle *pcap.Handle
	var err error
	if args.f != "" {
		log.Printf("Reading from pcap dump %q", args.f)
		handle, err = pcap.OpenOffline(args.f)
	} else {
		log.Printf("Starting capture on interface %q and port %d", args.i, args.p)
		handle, err = pcap.OpenLive(args.i, 4096, true, pcap.BlockForever)
	}

	if err != nil {
		return nil, err
	}

	expr := fmt.Sprintf("tcp and port %d", args.p)
	if err = handle.SetBPFFilter(expr); err != nil {
		return nil, err
	}

	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
}
