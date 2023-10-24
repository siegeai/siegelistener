package listener

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/siegeai/siegelistener/httpassembly"
	"github.com/siegeai/siegelistener/infer"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Listener struct {
	source          PacketSource
	publishDuration time.Duration
	publishTime     time.Time
	requestLogs     chan *RequestLog
	schemaMetrics   map[Schema]*SchemaMetrics
	endpointMetrics map[Endpoint]*EndpointMetrics
	registry        *prometheus.Registry
}

type Endpoint struct {
	Path   string
	Method string
}

type EndpointMetrics struct {
	Endpoint        Endpoint
	RequestDuration prometheus.Histogram
	RequestCount    prometheus.Counter
	ErrorCount      prometheus.Counter
}

type Schema string

type SchemaMetrics struct {
	UpdateTime  time.Time
	UpdateCount int
}

func NewEndpointMetrics(e Endpoint) *EndpointMetrics {
	namespace := "siege"
	subsystem := "listener"
	labels := prometheus.Labels{"path": e.Path, "method": e.Method}

	requestDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "request_duration",
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: labels,
	})

	requestCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "request_count",
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: labels,
	})

	errorCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "error_count",
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: labels,
	})

	return &EndpointMetrics{
		Endpoint:        e,
		RequestDuration: requestDuration,
		RequestCount:    requestCount,
		ErrorCount:      errorCount,
	}
}

func (m *EndpointMetrics) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		m.RequestCount,
		m.ErrorCount,
		m.RequestDuration,
	)
}

func (m *EndpointMetrics) HandleRequestLog(r *RequestLog) {
	if m.Endpoint != r.Endpoint {
		log := getLogger()
		log.Error("hit the wrong metrics", "lhs", m.Endpoint, "rhs", r.Endpoint)
		return
	}

	m.RequestCount.Inc()

	if 500 <= r.Code && r.Code <= 599 {
		m.ErrorCount.Inc()
	}

	m.RequestDuration.Observe(r.Duration.Seconds())
}

func (l *Listener) getOrCreateEndpointMetrics(e Endpoint) *EndpointMetrics {
	if m, ok := l.endpointMetrics[e]; ok {
		return m
	}
	m := NewEndpointMetrics(e)
	m.Register(l.registry)
	l.endpointMetrics[e] = m
	return m
}

func (l *Listener) getOrCreateSchemaMetrics(s Schema) *SchemaMetrics {
	if m, ok := l.schemaMetrics[s]; ok {
		return m
	}
	m := &SchemaMetrics{UpdateTime: time.Time{}, UpdateCount: 0}
	l.schemaMetrics[s] = m
	return m
}

func (l *Listener) handleRequestLog(r *RequestLog) {
	sm := l.getOrCreateSchemaMetrics(r.Schema)
	sm.UpdateTime = time.Now()
	sm.UpdateCount += 1

	em := l.getOrCreateEndpointMetrics(r.Endpoint)
	em.HandleRequestLog(r)
}

func (l *Listener) publish() {
	log := getLogger()
	log.Debug("publish")
	defer log.Debug("publish done")

	schemas := make([]Schema, 0, 10)
	for s, sm := range l.schemaMetrics {
		if l.publishTime.Before(sm.UpdateTime) {
			schemas = append(schemas, s)
		}
	}

	metrics, err := l.registry.Gather()
	if err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	enc := expfmt.NewEncoder(buf, expfmt.FmtText)
	for _, mf := range metrics {
		if err := enc.Encode(mf); err != nil {
			log.Error("could not encode metric family")
		}
	}

	req := make(map[string]any)
	req["listenerID"] = "424242"
	req["schemas"] = schemas
	req["metrics"] = buf.String()

	bs, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	log.Debug("publishing metrics", "payload", string(bs))

	resp, err := http.Post(
		"http://localhost:3000/api/v1/listener/update",
		"application/json",
		bytes.NewReader(bs),
	)

	if err != nil {
		log.Warn("could not send doc to server", "err", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Warn("error status sending doc to server", "status", resp.Status)
		return
	}

	l.publishTime = time.Now()
}

type RequestLog struct {
	Endpoint Endpoint
	Code     int
	Duration time.Duration
	Schema   Schema
}

func NewListener(source PacketSource) *Listener {
	return &Listener{
		source:          source,
		publishDuration: 5 * time.Second,
		publishTime:     time.Time{},
		requestLogs:     make(chan *RequestLog),
		schemaMetrics:   make(map[Schema]*SchemaMetrics),
		endpointMetrics: make(map[Endpoint]*EndpointMetrics),
		registry:        prometheus.NewRegistry(),
	}
}

func (l *Listener) ListenJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	f := &factory{l: l, ctx: ctx, wg: wg}
	assembler := httpassembly.NewAssembler(f)
	flushTicker := time.Tick(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return

		case packet := <-l.source.Packets():
			assembler.Assemble(packet)

		case <-flushTicker:
			assembler.FlushOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

func (l *Listener) PublishJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	publishTicker := time.Tick(l.publishDuration)
	for {
		select {
		case <-ctx.Done():
			return

		case r := <-l.requestLogs:
			l.handleRequestLog(r)

		case <-publishTicker:
			l.publish()
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

	log := getLogger()
	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
	if err != nil {
		log.Error("could not read request", "err", err, "len", len(req), "head", string(req[:min(len(req), 32)]))
		return
	}

	w, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(res)), r)
	if err != nil {
		log.Error("could not read response", "err", err, "len", len(res), "head", string(res[:min(len(res), 32)]))
		return
	}

	log.Info("handling", "method", r.Method, "path", r.URL.Path, "status", w.Status)

	rb, err := readAllEncoded(r.Header.Get("Content-Encoding"), r.Body)
	if err != nil {
		log.Error("could not read request body", "err", err, "len", len(req), "head", string(req[:min(len(req), 32)]))
		return
	}

	wb, err := readAllEncoded(w.Header.Get("Content-Encoding"), w.Body)
	if err != nil {
		log.Error("could not read response body", "err", err, "len", len(res), "head", string(res[:min(len(res), 32)]))
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
		panic("Unknown request method")
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

	path := strings.Join(resparts, "/")
	ps := openapi3.Paths{path: &pathItem}
	bs, err := json.Marshal(ps)
	if err != nil {
		panic(err)
	}

	log := getLogger()
	log.Debug("enqueuing request log")

	l.requestLogs <- &RequestLog{
		Endpoint: Endpoint{Path: path, Method: req.inner.Method},
		Code:     res.inner.StatusCode,
		Duration: 0,
		Schema:   Schema(bs),
	}
}

func handleRequestResponseProcRequestBody(req *request, res *response) *openapi3.RequestBodyRef {
	if len(req.body) == 0 {
		// maybe still track content type?
		return nil
	}

	log := getLogger()
	mt := openapi3.NewMediaType()
	if strings.Contains(req.inner.Header.Get("Content-Type"), "json") {
		sch, err := infer.ParseSampleBodyBytes(req.body)
		if err != nil {
			log.Warn("error parsing request body as json", "err", err)
			// could not parse json?
			return nil
		}
		mt.Schema = sch.NewRef()
	}

	rb := openapi3.NewRequestBody()
	rb.Content = openapi3.Content{}
	rb.Content[req.inner.Header.Get("Content-Type")] = mt
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

	log := getLogger()

	mt := openapi3.NewMediaType()
	if strings.Contains(res.inner.Header.Get("Content-Type"), "json") {
		sch, err := infer.ParseSampleBodyBytes(res.body)
		if err != nil {
			log.Warn("error parsing response body as json", "err", err)
			return nil
		}
		mt.Schema = sch.NewRef()
	}

	rs := openapi3.NewResponse()
	rs.Content = openapi3.Content{}
	rs.Content[res.inner.Header.Get("Content-Type")] = mt

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

func getLogger() *slog.Logger {
	return slog.Default()
}
