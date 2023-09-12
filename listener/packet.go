package main

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"log"
)

type newPacketSourceArgs struct {
	f string // filename
	i string // interface
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

func newPacketSource(opts ...newPacketSourceOpt) (*gopacket.PacketSource, error) {
	var args newPacketSourceArgs
	for _, opt := range opts {
		opt(&args)
	}
	if args.i == "" && args.f == "" {
		args.i = "lo"
	}

	var handle *pcap.Handle
	var err error
	if args.f != "" {
		log.Printf("Reading from pcap dump %q", args.f)
		handle, err = pcap.OpenOffline(args.f)
	} else {
		log.Printf("Starting capture on interface %q", args.i)
		handle, err = pcap.OpenLive(args.i, 4096, true, pcap.BlockForever)
	}

	if err != nil {
		return nil, err
	}

	if err = handle.SetBPFFilter("tcp and port 80"); err != nil {
		return nil, err
	}

	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
}
