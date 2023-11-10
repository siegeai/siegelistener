package listener

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/siegeai/siegelistener/httpassembly"
	"github.com/siegeai/siegelistener/infer"
	"github.com/siegeai/siegelistener/integrations/siegeserver"
)

type Listener struct {
	ListenerID      string
	publishInterval time.Duration
	requestLogs     chan *RequestLog
	schemasToSend   []string
	schemasSeen     map[[md5.Size]byte]struct{}
	responseMetrics map[ResponseMetricsKey]*ResponseMetrics
	registry        *prometheus.Registry
	source          PacketSource
	Assembler       *httpassembly.HttpAssembler
	Client          *siegeserver.Client
	Log             *slog.Logger
}

func NewListener(source PacketSource, client *siegeserver.Client) (*Listener, error) {
	f := &factory{l: nil}
	assembler := httpassembly.NewAssembler(f)

	listener := &Listener{
		source:          source,
		publishInterval: 15 * time.Second,
		requestLogs:     make(chan *RequestLog),
		schemasToSend:   nil,
		schemasSeen:     make(map[[md5.Size]byte]struct{}),
		responseMetrics: make(map[ResponseMetricsKey]*ResponseMetrics),
		registry:        prometheus.NewRegistry(),
		Assembler:       assembler,
		Client:          client,
		Log:             slog.Default(),
	}

	// circular dependency cringe
	f.l = listener

	return listener, nil
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
	sum := md5.Sum([]byte(r.Schema))
	if _, in := l.schemasSeen[sum]; !in {
		l.schemasSeen[sum] = struct{}{}
		l.schemasToSend = append(l.schemasToSend, string(r.Schema))
	}

	rm := l.getOrCreateResponseMetrics(ResponseMetricsKey{
		Path:   r.Path,
		Method: r.Method,
		Status: r.Status,
	})

	rm.HandleRequestLog(r)
}

func (l *Listener) encodeMetrics() (string, error) {
	mfs, err := l.registry.Gather()
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	enc := expfmt.NewEncoder(buf, expfmt.FmtText)
	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
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
	Schema   []byte
}

func (l *Listener) RegisterStartup() error {
	// TODO this should retry until it succeeds or the program is killed
	// TODO this should be used to validate the api key, if the key is bad we'll just
	//  shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	config, err := l.Client.Startup(ctx)
	if err != nil {
		return err
	}

	l.ListenerID = config.ListenerID
	l.Log = l.Log.With("listenerID", l.ListenerID)
	l.Log.Debug("listener/startup")
	return nil
}

func (l *Listener) RegisterShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := l.Client.Shutdown(ctx, l.ListenerID)
	if err != nil {
		l.Log.Error("could not register shutdown", "err", err)
		return
	}

	l.Log.Debug("listener/shutdown")
}

func (l *Listener) publish() {
	metrics, err := l.encodeMetrics()
	if err != nil {
		panic(err)
	}

	update := siegeserver.ListenerUpdate{
		ListenerID: l.ListenerID,
		Schemas:    l.schemasToSend,
		Metrics:    metrics,
	}

	err = l.Client.Update(context.Background(), update)
	if err != nil {
		l.Log.Error("listener/update failed", "err", err)
		return
	}

	l.schemasToSend = nil
	l.Log.Debug("listener/update")
}

func (l *Listener) ListenJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	l.Log.Debug("listen job start")
	defer l.Log.Debug("listen job end")

	flushTicker := time.NewTicker(time.Minute)
	defer flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case packet := <-l.source.Packets():
			l.Assembler.Assemble(packet)

		case <-flushTicker.C:
			l.Assembler.FlushCloseOlderThan(time.Now().Add(time.Minute * -2))
		}
	}
}

func (l *Listener) PublishJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	l.Log.Debug("publish job start")
	defer l.Log.Debug("publish job end")

	publishTicker := time.NewTicker(l.publishInterval)
	defer publishTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case r := <-l.requestLogs:
			l.handleRequestLog(r)

		case <-publishTicker.C:
			l.publish()
		}
	}
}

func (l *Listener) ReassembleJob(ctx context.Context, wg *sync.WaitGroup) {
	l.Assembler.ReassembleJob(ctx, wg)
}

type factory struct {
	l *Listener
}

func (f *factory) New() httpassembly.HttpStream {
	return &stream{
		Listener: f.l,
		Log:      slog.Default(),
	}
}

type stream struct {
	Listener *Listener
	Log      *slog.Logger
}

func (s *stream) ReassembledRequestResponse(req []byte, res []byte, duration float64) {

	// TODO payload value here is gross, duration parameter is gross, where could those come from?
	// TODO needs to stress test and make sure the listener doesn't fkn die or eat all the memory
	//  run for a long time with a lot of traffic?
	//  watch mem use.

	// mb
	payload := float64(len(req)+len(res)) / (1000.0 * 1000.0)

	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
	if err != nil {
		s.Log.Error("could not read request", "err", err)
		return
	}

	w, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(res)), r)
	if err != nil {
		s.Log.Error("could not read response", "err", err)
		return
	}

	// Skip anything that isn't JSON
	if !strings.Contains(r.Header.Get("Content-Type"), "json") && !strings.Contains(w.Header.Get("Content-Type"), "json") {
		s.Log.Debug("skipped", "method", r.Method, "path", r.URL.Path, "status", w.Status)
		return
	}

	rb, err := readAllEncoded(r.Header.Get("Content-Encoding"), r.Body)
	if err != nil {
		s.Log.Error("could not read request body", "err", err, "method", r.Method, "path", r.URL.Path, "status", w.Status)
		return
	}

	wb, err := readAllEncoded(w.Header.Get("Content-Encoding"), w.Body)
	if err != nil {
		s.Log.Error("could not read response body", "err", err, "method", r.Method, "path", r.URL.Path, "status", w.Status)
		return
	}

	u := request{inner: r, body: rb}
	v := response{inner: w, body: wb}
	s.Listener.handleRequestResponse(&u, &v, payload, duration)
	s.Log.Debug("handled", "method", r.Method, "path", r.URL.Path, "status", w.Status)
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

func (l *Listener) handleRequestResponse(req *request, res *response, payload float64, duration float64) {

	op := openapi3.Operation{}
	// header info is bloating schemas a lot, probably want to track across the api as
	// a whole instead of just per endpoint? or track separately? Not sure about tracking
	// every combination of header.
	//
	//for k, _ := range req.inner.Header {
	//	p := &openapi3.ParameterRef{Value: &openapi3.Parameter{
	//		Name: k,
	//		In:   "header",
	//	}}
	//
	//	op.Parameters = append(op.Parameters, p)
	//}
	op.RequestBody = l.handleRequestResponseProcRequestBody(req, res)
	op.Responses = l.handleRequestResponseProcResponses(req, res)

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

	l.Log.Debug("enqueuing request log")

	l.requestLogs <- &RequestLog{
		Path:     path,
		Method:   req.inner.Method,
		Status:   res.inner.StatusCode,
		Duration: duration,
		Payload:  payload,
		Schema:   bs,
	}
}

func (l *Listener) handleRequestResponseProcRequestBody(req *request, res *response) *openapi3.RequestBodyRef {
	if len(req.body) == 0 {
		// maybe still track content type?
		return nil
	}

	mt := openapi3.NewMediaType()
	if strings.Contains(req.inner.Header.Get("Content-Type"), "json") {
		sch, err := infer.ParseSampleBodyBytes(req.body)
		if err != nil {
			l.Log.Warn("error parsing request body as json", "err", err)
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

func (l *Listener) handleRequestResponseProcResponses(req *request, res *response) openapi3.Responses {
	if len(res.body) == 0 {
		r := openapi3.Responses{
			strconv.Itoa(res.inner.StatusCode): &openapi3.ResponseRef{
				Value: openapi3.NewResponse().WithDescription(""),
			},
		}
		return r
	}

	mt := openapi3.NewMediaType()
	if strings.Contains(res.inner.Header.Get("Content-Type"), "json") {
		sch, err := infer.ParseSampleBodyBytes(res.body)
		if err != nil {
			l.Log.Warn("error parsing response body as json", "err", err)
			return nil
		}
		mt.Schema = sch.NewRef()
	}

	rs := openapi3.NewResponse()
	rs.Content = openapi3.Content{}
	rs.Content[res.inner.Header.Get("Content-Type")] = mt

	// same thoughts as the request headers
	//if len(res.inner.Header) > 0 {
	//	rs.Headers = openapi3.Headers{}
	//}
	//
	//for k, _ := range res.inner.Header {
	//	rs.Headers[k] = &openapi3.HeaderRef{Value: &openapi3.Header{}}
	//}

	return openapi3.Responses{
		strconv.Itoa(res.inner.StatusCode): &openapi3.ResponseRef{Value: rs},
	}
}
