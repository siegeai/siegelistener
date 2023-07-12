package main

import (
	"bufio"
	"bytes"
	"flag"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"io"
	"log"
	"net/http"
	"time"
)

type httpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

func (h *httpStream) run() {

	var buffer bytes.Buffer
	tee := io.TeeReader(&h.r, &buffer)

	for {
		// For some reason doing request before response doesn't work (as written)

		resBuf := bufio.NewReader(tee)
		res, resErr := http.ReadResponse(resBuf, nil)
		if resErr == io.ErrUnexpectedEOF {
			// We must read until we see an EOF... very important!
			// continue
		} else if resErr != nil {
			// log.Println("Error reading stream", h.net, h.transport, ":", err)
		} else {
			body, resErr := io.ReadAll(res.Body)
			defer res.Body.Close()
			if resErr != nil {
				log.Println("Res error reading body:", resErr)
			} else {
				log.Println(h.transport, res.Status, "len(body) =", len(body))
			}
		}

		reqBuf := bufio.NewReader(&buffer)
		req, reqErr := http.ReadRequest(reqBuf)
		if reqErr == io.EOF {
			// We must read until we see an EOF... very important!
			// continue
		} else if reqErr != nil {
			// log.Println("Error reading stream", h.net, h.transport, ":", err)
		} else {
			body, reqErr := io.ReadAll(req.Body)
			defer req.Body.Close()
			if reqErr != nil {
				log.Println("Req error reading body:", reqErr)
			} else {
				log.Println(h.transport, req.Method, req.URL, "len(body) =", len(body))
			}
		}
	}
}

type httpStreamFactory struct{}

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	stream := &httpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go stream.run()
	return &stream.r
}

func isTcpPacket(p gopacket.Packet) bool {
	return p != nil &&
		p.NetworkLayer() != nil &&
		p.TransportLayer() != nil &&
		p.TransportLayer().LayerType() == layers.LayerTypeTCP
}

func main() {
	var iface = flag.String("i", "eth0", "Interface to get packets from")
	var fname = flag.String("r", "", "Filename to read from, overrides -i")
	flag.Parse()

	var handle *pcap.Handle
	var err error
	if *fname != "" {
		log.Printf("Reading from pcap dump %q", *fname)
		handle, err = pcap.OpenOffline(*fname)
	} else {
		log.Printf("Starting capture on interface %q", *iface)
		handle, err = pcap.OpenLive(*iface, 4096, true, pcap.BlockForever)
	}

	if err != nil {
		log.Fatal(err)
	}

	if err := handle.SetBPFFilter("tcp and port 80"); err != nil {
		log.Fatal(err)
	}

	streamFactory := &httpStreamFactory{}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.Tick(time.Minute)

	log.Println("reading in packets")
	for {
		select {
		case packet := <-packets:
			// If the filter only picks up tcp packets maybe we don't actually need this
			if !isTcpPacket(packet) {
				continue
			}

			//log.Println("Got a packet", packet.Dump())
			//log.Println("Got a packet")
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}
