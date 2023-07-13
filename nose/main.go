package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

// TODO stupid name
type biflow struct {
	ipv4 gopacket.Flow
	tcp  gopacket.Flow
}

func (b *biflow) reverse() biflow {
	return biflow{ipv4: b.ipv4.Reverse(), tcp: b.tcp.Reverse()}
}

// TODO stupid name, should this hold ptrs?
type context struct {
	req map[biflow]request
	res map[biflow]response
}

func newContext() *context {
	return &context{
		req: make(map[biflow]request),
		res: make(map[biflow]response),
	}
}

// TODO stupid name, parsedRequest?
type request struct {
	inner *http.Request
	body  []byte
}

// TODO stupid name, parsedResponse?
type response struct {
	inner *http.Response
	body  []byte
}

type httpStream struct {
	a *context
	b biflow
	r tcpreader.ReaderStream
}

func (h *httpStream) run() {
	// TODO how do I know if its time for this to return?
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
		handled, _ := handleRequest(h, r)
		if handled {
			return
		}

		// Reset the reader's bytes for a second pass
		r.Reset(bytes.NewReader(buffer.Bytes()))

		// Try to process the buffer as a response
		handled, resErr := handleResponse(h, r)
		if resErr != nil {
			return
		}
		if handled {
			return
		}

		return
	}
}

func handleRequest(h *httpStream, reader *bufio.Reader) (handled bool, err error) {
	req, err := http.ReadRequest(reader)
	if err != nil {
		return false, err
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return true, err
	}
	defer req.Body.Close()

	r := request{inner: req, body: body}
	d := h.b.reverse()
	if res, exists := h.a.res[d]; exists {
		handleRequestResponse(h.a, &r, &res)
		delete(h.a.res, d)
	} else {
		_, exists := h.a.req[h.b]
		if exists {
			log.Fatal("Multiple requests before getting a response. Need a queue?")
		}
		h.a.req[h.b] = r
	}
	return true, nil
}

func handleResponse(h *httpStream, reader *bufio.Reader) (handled bool, err error) {
	res, err := http.ReadResponse(reader, nil)
	if err != nil {
		return false, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return true, err
	}
	defer res.Body.Close()

	r := response{inner: res, body: body}
	d := h.b.reverse()
	if req, exists := h.a.req[d]; exists {
		handleRequestResponse(h.a, &req, &r)
		delete(h.a.req, d)
	} else {
		_, exists := h.a.res[h.b]
		if exists {
			log.Fatal("Multiple responses before getting a request. Need a queue?")
		}
		h.a.res[h.b] = r
	}
	return true, nil
}

func handleRequestResponse(a *context, req *request, res *response) {
	dump, err := httputil.DumpRequest(req.inner, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", string(dump))
	fmt.Printf("%s\n\n", string(req.body))

	dump, err = httputil.DumpResponse(res.inner, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", string(dump))
	fmt.Printf("%s\n", string(res.body))
}

type httpStreamFactory struct {
	a *context
}

func (h *httpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	s := &httpStream{
		a: h.a,
		b: biflow{ipv4: net, tcp: transport},
		r: tcpreader.NewReaderStream(),
	}
	go s.run()
	return &s.r
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

	streamFactory := &httpStreamFactory{a: newContext()}
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

			log.Println("Got a packet", packet.Dump())
			//log.Println("Got a packet")
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}
