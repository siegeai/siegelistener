package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/google/uuid"
	"github.com/siegeai/siegelistener/infer"
	"github.com/siegeai/siegelistener/merge"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type listener struct {
	source     *gopacket.PacketSource
	a          *context // the stupidest field
	doc        *openapi3.T
	mergeQueue chan *openapi3.T
}

func newListener(source *gopacket.PacketSource) listener {
	return listener{
		source:     source,
		a:          newContext(),
		mergeQueue: make(chan *openapi3.T, 10),
	}
}

func (l *listener) run() error {

	streamPool := tcpassembly.NewStreamPool(l)
	assembler := tcpassembly.NewAssembler(streamPool)

	packets := l.source.Packets()
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

		case doc := <-l.mergeQueue:
			l.doc = merge.Doc(l.doc, doc)
			bs, err := json.Marshal(l.doc)
			if err != nil {
				log.Println(err)
			} else {
				log.Println(string(bs))
				body := bytes.NewBuffer(bs)
				resp, err := http.Post("http://localhost:3000/api/v1/spec.json", "application/json", body)
				if err != nil {
					log.Printf("Could not send request because %v", err)
				} else {
					if resp.StatusCode != http.StatusOK {
						log.Printf("Unexpected response %v", resp)
					}
				}
			}

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

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
	l *listener
	a *context
	b biflow
	r tcpreader.ReaderStream
}

func (h *httpStream) run() {
	// TODO how do I know if its time for this to return?
	buffer := bytes.NewBuffer(make([]byte, 0, 4096))
	for {
		// TODO seems to be a bug when the Connection: keep-alive param is set.

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
		handleRequestResponse(h.l, &r, &res)
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
		handleRequestResponse(h.l, &req, &r)
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

func handleRequestResponse(l *listener, req *request, res *response) {
	log.Println("handling", req.inner.Method, req.inner.URL, "->", res.inner.Status)
	if 500 <= res.inner.StatusCode && res.inner.StatusCode < 600 {
		return
	}

	op := openapi3.Operation{}
	for k, _ := range req.inner.Header {
		p := &openapi3.ParameterRef{Value: &openapi3.Parameter{
			Name: k,
			In:   "header",
		}}

		op.Parameters = append(op.Parameters, p)
	}
	op.RequestBody = handleRequestResponseProcRequestBody(req, res)
	op.Responses = handleRequestResponseProcResponses(req, res)

	pathItem := openapi3.PathItem{}
	switch req.inner.Method {
	case http.MethodConnect:
		pathItem.Connect = &op
	case http.MethodDelete:
		pathItem.Delete = &op
	case http.MethodGet:
		pathItem.Get = &op
	case http.MethodHead:
		pathItem.Head = &op
	case http.MethodOptions:
		pathItem.Options = &op
	case http.MethodPatch:
		pathItem.Patch = &op
	case http.MethodPost:
		pathItem.Post = &op
	case http.MethodPut:
		pathItem.Put = &op
	case http.MethodTrace:
		pathItem.Trace = &op
	default:
		log.Fatal("Unknown request method")
	}

	nparams := 1
	parts := strings.Split(req.inner.URL.Path, "/")
	resparts := make([]string, len(parts))
	for i, p := range parts {
		if _, err := strconv.Atoi(p); err == nil {
			resparts[i] = fmt.Sprintf("{arg%d}", nparams)
			a := &openapi3.ParameterRef{Value: &openapi3.Parameter{
				Name:   fmt.Sprintf("arg%d", nparams),
				In:     "path",
				Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "integer"}},
			}}
			op.Parameters = append(op.Parameters, a)
			nparams += 1
		} else if _, err := uuid.Parse(p); err == nil {
			resparts[i] = fmt.Sprintf("{arg%d}", nparams)
			a := &openapi3.ParameterRef{Value: &openapi3.Parameter{
				Name:   fmt.Sprintf("arg%d", nparams),
				In:     "path",
				Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string", Format: "uuid"}},
			}}
			op.Parameters = append(op.Parameters, a)
			nparams += 1
		} else {
			resparts[i] = p
		}
	}

	ps := openapi3.Paths{strings.Join(resparts, "/"): &pathItem}

	l.mergeQueue <- &openapi3.T{
		OpenAPI: "3.0.0",
		Info:    &openapi3.Info{Title: "Example", Version: "0.0.1"},
		Paths:   ps,
	}
}

func handleRequestResponseProcRequestBody(req *request, res *response) *openapi3.RequestBodyRef {
	if len(req.body) == 0 {
		return nil
	}

	if res.inner.StatusCode == 400 || (500 <= res.inner.StatusCode && res.inner.StatusCode < 600) {
		return nil
	}

	sch, err := infer.ParseSampleBodyBytes(req.body)
	if err != nil {
		// could not parse json?
		return nil
	}

	mt := openapi3.NewMediaType()
	mt.Schema = sch.NewRef()

	rb := openapi3.NewRequestBody()
	rb.Content = openapi3.Content{}
	rb.Content["application/json"] = mt

	return &openapi3.RequestBodyRef{Value: rb}
}

func handleRequestResponseProcResponses(req *request, res *response) openapi3.Responses {
	if len(res.body) == 0 {
		r := openapi3.Responses{
			strconv.Itoa(res.inner.StatusCode): &openapi3.ResponseRef{
				Value: openapi3.NewResponse().WithDescription(""),
			},
		}
		return r
	}

	sch, err := infer.ParseSampleBodyBytes(res.body)
	if err != nil {
		// could not parse json?
		return nil
	}

	mt := openapi3.NewMediaType()
	mt.Schema = sch.NewRef()

	rs := openapi3.NewResponse()
	rs.Content = openapi3.Content{}
	rs.Content["application/json"] = mt

	if len(res.inner.Header) > 0 {
		rs.Headers = openapi3.Headers{}
	}

	for k, _ := range res.inner.Header {
		rs.Headers[k] = &openapi3.HeaderRef{Value: &openapi3.Header{}}
	}

	return openapi3.Responses{
		strconv.Itoa(res.inner.StatusCode): &openapi3.ResponseRef{Value: rs},
	}
}

func (l *listener) New(netFlow, tcpFlow gopacket.Flow) tcpassembly.Stream {
	s := &httpStream{
		l: l,
		a: l.a,
		b: biflow{ipv4: netFlow, tcp: tcpFlow},
		r: tcpreader.NewReaderStream(),
	}
	go s.run()
	return &s.r
}
