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
	"github.com/valyala/fastjson"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	log.Println("handling", req.inner.Method, req.inner.URL, "->", res.inner.Status)

	arena := &fastjson.Arena{}

	spec := arena.NewObject()
	path := arena.NewObject()
	op := arena.NewObject()
	resp := arena.NewObject()
	cnt := arena.NewObject()
	ajs := arena.NewObject()
	scm := arena.NewObject()

	// Pain point, how to detect query params vs paths?
	spec.Set(req.inner.URL.Path, path)
	path.Set(strings.ToLower(req.inner.Method), op)
	op.Set("responses", resp)

	if len(req.body) > 0 {
		reqVal, err := fastjson.ParseBytes(req.body)
		if err != nil {
			log.Println("could not parse request body because", err)
		} else {
			reqSchema, err := genSchema(arena, reqVal)
			if err != nil {
				log.Fatal(err)
			}

			op.Set("requestBody", reqSchema)
		}
	}

	resp.Set(strconv.Itoa(res.inner.StatusCode), cnt)
	cnt.Set("content", ajs)

	if len(res.body) > 0 {
		resVal, err := fastjson.ParseBytes(res.body)
		if err != nil {
			log.Println("could not parse response body because", err)
		} else {
			resSchema, err := genSchema(arena, resVal)
			if err != nil {
				log.Fatal(err)
			}

			ajs.Set("application/json", scm)
			scm.Set("schema", resSchema)
		}
	}

	log.Println(spec)
}

func genSchema(a *fastjson.Arena, v *fastjson.Value) (*fastjson.Value, error) {
	// how to detect recursive objects?
	// how to get a list's type?
	// can we under-sample list items?
	// can we do more invasive string type checks but only for a few list items?

	switch v.Type() {
	case fastjson.TypeNull:
		// wtf should this be?
		// if it's a field only seen once we don't know what it is other than nullable
		// if we see the field again in a list we may get lucky and get a type
		obj := a.NewNull()
		return obj, nil

	case fastjson.TypeObject:
		obj := a.NewObject()
		objProps := a.NewObject()
		obj.Set("type", a.NewString("object"))
		obj.Set("properties", objProps)

		vobj, err := v.Object()
		if err != nil {
			return nil, err
		}

		var visitErr *error = nil

		vobj.Visit(func(key []byte, vv *fastjson.Value) {
			if visitErr != nil {
				return
			}
			prop, err := genSchema(a, vv)
			if err != nil {
				visitErr = &err
			}
			objProps.Set(string(key), prop)
		})

		if visitErr != nil {
			return nil, *visitErr
		}

		return obj, nil
	case fastjson.TypeArray:
		// need to compute a union type of all elements in the array
		obj := a.NewObject()
		obj.Set("type", a.NewString("array"))
		return obj, nil
	case fastjson.TypeString:
		// quick check against known "string" types
		// e.g., numeric, email, etc
		obj := a.NewObject()
		obj.Set("type", a.NewString("string"))
		return obj, nil
	case fastjson.TypeNumber:
		// integer or number?
		obj := a.NewObject()
		obj.Set("type", a.NewString("number")) // integer?
		return obj, nil
	case fastjson.TypeTrue:
		obj := a.NewObject()
		obj.Set("type", a.NewString("boolean"))
		return obj, nil
	case fastjson.TypeFalse:
		obj := a.NewObject()
		obj.Set("type", a.NewString("boolean"))
		return obj, nil
	}

	panic("Unknown v.Type()")
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

			// TODO
			//  - postman sends Connection: keep-alive by default which fucks with our
			//    assembly. Need to fix.
			//  - http2?

			//log.Println("Got a packet", packet.Dump())
			//log.Println("Got a packet")
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}
