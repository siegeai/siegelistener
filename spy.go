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
	buffer := bytes.NewBuffer(make([]byte, 0, 4096))
	for {
		// Read into buffer to unblock the tcp assembler ASAP. Can I do this without
		// the intermediate buffer alloc in Copy?
		_, err := io.Copy(buffer, &h.r)
		if err != nil {
			return
		}

		// Set up a bufio.Reader because http expects it. This will alloc memory and it
		// makes me very mad.
		r := bufio.NewReaderSize(bytes.NewReader(buffer.Bytes()), buffer.Len())

		// Try to process the buffer as a request
		handled, _ := handleRequest(r)
		if handled {
			return
		}

		// Reset the reader's bytes for a second pass
		r.Reset(bytes.NewReader(buffer.Bytes()))

		// Try to process the buffer as a response
		handled, resErr := handleResponse(r)
		if resErr != nil {
			return
		}
		if handled {
			return
		}

		return
	}
}

func handleRequest(reader *bufio.Reader) (handled bool, err error) {
	req, err := http.ReadRequest(reader)
	if err != nil {
		return false, err
	}

	_, err = io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		return true, err
	}

	log.Println(req.Method, req.URL)
	return true, nil
}

func handleResponse(reader *bufio.Reader) (handled bool, err error) {
	res, err := http.ReadResponse(reader, nil)
	if err != nil {
		return false, err
	}

	_, err = io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return true, err
	}

	log.Println(res.Status)
	return true, nil
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
