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
	"github.com/siegeai/siegelistener/integrations/siegeserver"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Listener struct {
	ListenerID      string
	publishInterval time.Duration
	publishTime     time.Time
	requestLogs     chan *RequestLog
	schemaMetrics   map[Schema]*SchemaMetrics
	responseMetrics map[ResponseMetricsKey]*ResponseMetrics
	registry        *prometheus.Registry
	source          PacketSource
}

func NewListener(source PacketSource) *Listener {
	return &Listener{
		source:          source,
		publishInterval: 15 * time.Second,
		publishTime:     time.Time{},
		requestLogs:     make(chan *RequestLog),
		schemaMetrics:   make(map[Schema]*SchemaMetrics),
		responseMetrics: make(map[ResponseMetricsKey]*ResponseMetrics),
		registry:        prometheus.NewRegistry(),
	}
}

type ResponseMetricsKey struct {
	Path   string
	Method string
	Status int
}

// ResponseMetrics may be a lot to track per (path, method, status), especially since
// we don't have queries for each of these.
type ResponseMetrics struct {
	Path     string
	Method   string
	Status   int
	Total    prometheus.Counter
	Duration prometheus.Histogram
	Payload  prometheus.Histogram
}

func NewResponseMetrics(path string, method string, status int) *ResponseMetrics {
	f := NewPrometheusMetricFactory(path, method, status)
	return &ResponseMetrics{
		Path:     path,
		Method:   method,
		Status:   status,
		Total:    f.NewCounter("http_response_total"),
		Duration: f.NewHistogram("http_response_duration_s"),
		Payload:  f.NewHistogram("http_response_payload_mb"),
	}
}

func (m *ResponseMetrics) Register(r prometheus.Registerer) {
	r.MustRegister(m.Total, m.Duration, m.Payload)
}

func (m *ResponseMetrics) HandleRequestLog(r *RequestLog) {
	// sense check
	if m.Path != r.Path || m.Method != r.Method || m.Status != r.Status {
		panic("these metrics are not for the given request log")
	}

	m.Total.Inc()
	m.Duration.Observe(r.Duration)
	m.Payload.Observe(r.Payload)
}

type PrometheusMetricFactory struct {
	Namespace string
	Subsystem string
	Labels    prometheus.Labels
}

func NewPrometheusMetricFactory(path string, method string, status int) PrometheusMetricFactory {
	namespace := "siege"
	subsystem := "listener"
	labels := prometheus.Labels{"path": path, "method": method, "status": strconv.Itoa(status)}
	return PrometheusMetricFactory{Namespace: namespace, Subsystem: subsystem, Labels: labels}
}

func (f *PrometheusMetricFactory) NewCounter(name string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name:        name,
		Namespace:   f.Namespace,
		Subsystem:   f.Subsystem,
		ConstLabels: f.Labels,
	})
}

func (f *PrometheusMetricFactory) NewHistogram(name string) prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        name,
		Namespace:   f.Namespace,
		Subsystem:   f.Subsystem,
		ConstLabels: f.Labels,
	})
}

type Schema string

type SchemaMetrics struct {
	UpdateTime  time.Time
	UpdateCount int
}

func (l *Listener) getOrCreateSchemaMetrics(s Schema) *SchemaMetrics {
	m, in := l.schemaMetrics[s]
	if in {
		return m
	}
	m = &SchemaMetrics{UpdateTime: time.Time{}, UpdateCount: 0}
	l.schemaMetrics[s] = m
	return m
}

func (l *Listener) getOrCreateResponseMetrics(key ResponseMetricsKey) *ResponseMetrics {
	m, in := l.responseMetrics[key]
	if in {
		return m
	}
	m = NewResponseMetrics(key.Path, key.Method, key.Status)
	m.Register(l.registry)
	l.responseMetrics[key] = m
	return m
}

func (l *Listener) handleRequestLog(r *RequestLog) {
	sm := l.getOrCreateSchemaMetrics(r.Schema)
	sm.UpdateTime = time.Now()
	sm.UpdateCount += 1

	rm := l.getOrCreateResponseMetrics(ResponseMetricsKey{
		Path:   r.Path,
		Method: r.Method,
		Status: r.Status,
	})

	rm.HandleRequestLog(r)
}

func (l *Listener) encodeMetrics() (string, error) {
	log := getLogger()

	mfs, err := l.registry.Gather()
	if err != nil {
		log.Error("gather metrics failed", "err", err)
		return "", err
	}

	buf := &bytes.Buffer{}
	enc := expfmt.NewEncoder(buf, expfmt.FmtText)
	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
			log.Error("encode metrics failed", "err", err)
			return "", err
		}
	}

	return buf.String(), nil
}

type RequestLog struct {
	Path     string
	Method   string
	Status   int
	Duration float64
	Payload  float64
	Schema   Schema
}

func (l *Listener) RegisterStartup() error {
	// TODO this should retry until it succeeds or the program is killed
	// TODO this should be used to validate the api key, if the key is bad we'll just
	//  shutdown

	log := getLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	config, err := siegeserver.NewClient().Startup(ctx)
	if err != nil {
		log.Error("listener/startup failed", "err", err)
		return err
	}

	slog.Debug("listener/startup", "listenerID", config.ListenerID)
	l.ListenerID = config.ListenerID
	return nil
}

func (l *Listener) RegisterShutdown() {
	log := getLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := siegeserver.NewClient().Shutdown(ctx, l.ListenerID)
	if err != nil {
		log.Error("listener/shutdown failed", "err", err)
		return
	}

	slog.Debug("listener/shutdown", "listener_id", l.ListenerID)
}

func (l *Listener) publish() {
	log := getLogger()

	schemas := make([]string, 0, 10)
	for s, sm := range l.schemaMetrics {
		if l.publishTime.Before(sm.UpdateTime) {
			schemas = append(schemas, string(s))
		}
	}

	metrics, err := l.encodeMetrics()
	if err != nil {
		panic(err)
	}

	update := siegeserver.ListenerUpdate{
		ListenerID: l.ListenerID,
		Schemas:    schemas,
		Metrics:    metrics,
	}

	err = siegeserver.NewClient().Update(context.Background(), update)
	if err != nil {
		log.Error("listener/update failed", "err", err)
		return
	}

	l.publishTime = time.Now()
	log.Debug("listener/update")
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
			assembler.FlushCloseOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

func (l *Listener) PublishJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	publishTicker := time.Tick(l.publishInterval)
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

func (s *stream) ReassembledRequestResponse(req []byte, res []byte, duration float64) {

	// mb
	payload := float64(len(req)+len(res)) / (1000.0 * 1000.0)

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
	handleRequestResponse(s.l, &u, &v, payload, duration)
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

func handleRequestResponse(l *Listener, req *request, res *response, payload float64, duration float64) {

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
		Path:     path,
		Method:   req.inner.Method,
		Status:   res.inner.StatusCode,
		Duration: duration,
		Payload:  payload,
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
