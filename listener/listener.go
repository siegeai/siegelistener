package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/gopacket"
	"github.com/google/uuid"
	"github.com/siegeai/siegelistener/httpassembly"
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
	doc        *openapi3.T
	mergeQueue chan *openapi3.T
}

func newListener(source *gopacket.PacketSource) listener {
	return listener{
		source:     source,
		mergeQueue: make(chan *openapi3.T, 10),
	}
}

type factory struct {
	l *listener
}

func (f *factory) New() httpassembly.HttpStream {
	return &stream{l: f.l}
}

type stream struct {
	l *listener
}

func (s *stream) ReassembledRequestResponse(req *http.Request, res *http.Response) {

	r, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Could not read request body because %s", err)
		return
	}
	defer func() {
		if err := req.Body.Close(); err != nil {
			log.Printf("Could not close requset body because %s", err)
		}
	}()

	w, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Could not read response body because %s", err)
		return
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Could not close response body because %s", err)
		}
	}()

	u := request{inner: req, body: r}
	v := response{inner: res, body: w}
	handleRequestResponse(s.l, &u, &v)
}

func (l *listener) run() error {

	f := &factory{l: l}
	assembler := httpassembly.NewAssembler(f)

	packets := l.source.Packets()
	ticker := time.Tick(time.Minute)

	for {
		select {
		case packet := <-packets:
			assembler.Assemble(packet)

		case <-ticker:
			//assembler.FlushCloseOlderThan(time.Now().Add(time.Minute * -2))

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
		}
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
