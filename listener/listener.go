package listener

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/siegeai/siegelistener/httpassembly"
	"github.com/siegeai/siegelistener/infer"
	"github.com/siegeai/siegelistener/merge"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Listener struct {
	source     PacketSource
	doc        *openapi3.T
	mergeQueue chan *openapi3.T
}

func NewListener(source PacketSource) *Listener {
	return &Listener{
		source:     source,
		mergeQueue: make(chan *openapi3.T, 10),
	}
}

func (l *Listener) Listen(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("listener starting")
	defer slog.Info("listener done")

	childWg := &sync.WaitGroup{}
	defer childWg.Wait()

	f := &factory{l: l, ctx: ctx, wg: childWg}
	assembler := httpassembly.NewAssembler(f)
	ticker := time.Tick(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return

		case packet := <-l.source.Packets():
			assembler.Assemble(packet)

		case <-ticker:
			//assembler.FlushCloseOlderThan(time.Now().Add(time.Minute * -2))

		case doc := <-l.mergeQueue:
			l.doc = merge.Doc(l.doc, doc)
			bs, err := json.Marshal(l.doc)
			if err != nil {
				slog.Error("could not write doc json", "err", err)
				continue
			}

			slog.Info("updated doc", "doc", string(bs))
			body := bytes.NewBuffer(bs)
			resp, err := http.Post("http://localhost:3000/api/v1/spec.json", "application/json", body)
			if err != nil {
				slog.Warn("could not send doc to server", "err", err)
			} else {
				if resp.StatusCode != http.StatusOK {
					slog.Warn("error status sending doc to server", "status", resp.Status)
				}
			}
		}
	}
}

type factory struct {
	l   *Listener
	ctx context.Context
	wg  *sync.WaitGroup
}

func (f *factory) New() httpassembly.HttpStream {
	return &stream{l: f.l}
}

func (f *factory) Context() context.Context {
	return f.ctx
}

func (f *factory) WaitGroup() *sync.WaitGroup {
	return f.wg
}

type stream struct {
	l *Listener
}

func (s *stream) ReassembledRequestResponse(req []byte, res []byte) {

	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
	if err != nil {
		slog.Error("could not read request", "err", err, "len", len(req), "head", string(req[:min(len(req), 32)]))
		return
	}

	w, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(res)), r)
	if err != nil {
		slog.Error("could not read response", "err", err, "len", len(res), "head", string(res[:min(len(res), 32)]))
		return
	}

	slog.Info("handling", "method", r.Method, "path", r.URL.Path, "status", w.Status)
	// TODO want to handle request / response pairs where we've thrown away the body because its
	//  something other than an api call (e.g. html or a file).

	rb, err := readAllEncoded(r.Header.Get("Content-Encoding"), r.Body)
	if err != nil {
		slog.Error("could not read request body", "err", err, "len", len(req), "head", string(req[:min(len(req), 32)]))
		return
	}

	wb, err := readAllEncoded(w.Header.Get("Content-Encoding"), w.Body)
	if err != nil {
		slog.Error("could not read response body", "err", err, "len", len(res), "head", string(res[:min(len(res), 32)]))
		return
	}

	u := request{inner: r, body: rb}
	v := response{inner: w, body: wb}
	handleRequestResponse(s.l, &u, &v)
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

func handleRequestResponse(l *Listener, req *request, res *response) {
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
