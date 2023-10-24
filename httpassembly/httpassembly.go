package httpassembly

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HttpAssembler struct {
	pool      *reassembly.StreamPool
	assembler *reassembly.Assembler
}

func NewAssembler(factory HttpStreamFactory) *HttpAssembler {
	f := &factoryWrapper{
		wrap: factory,
	}
	p := reassembly.NewStreamPool(f)
	a := reassembly.NewAssembler(p)
	return &HttpAssembler{pool: p, assembler: a}
}

type assemblyContext struct {
	CaptureInfo gopacket.CaptureInfo
}

func (c *assemblyContext) GetCaptureInfo() gopacket.CaptureInfo {
	return c.CaptureInfo
}

func (a *HttpAssembler) Assemble(p gopacket.Packet) {
	tcp := p.Layer(layers.LayerTypeTCP)
	if tcp == nil {
		return
	}

	c := assemblyContext{CaptureInfo: p.Metadata().CaptureInfo}
	a.assembler.AssembleWithContext(p.NetworkLayer().NetworkFlow(), tcp.(*layers.TCP), &c)
}

func (a *HttpAssembler) FlushOlderThan(t time.Time) {
	a.assembler.FlushCloseOlderThan(t)
}

type HttpStreamFactory interface {
	New() HttpStream
	Context() context.Context
	WaitGroup() *sync.WaitGroup
}

type HttpStream interface {
	ReassembledRequestResponse(req []byte, res []byte)
}

type factoryWrapper struct {
	wrap    HttpStreamFactory
	counter int
}

func (f *factoryWrapper) New(netFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	w := f.wrap.New()

	fsmOptions := reassembly.TCPSimpleFSMOptions{SupportMissingEstablishment: false}
	s := streamWrapper{
		sid:  f.counter,
		wrap: w,
		fsm:  reassembly.NewTCPSimpleFSM(fsmOptions),
		opt:  reassembly.NewTCPOptionCheck(),
		sides: map[reassembly.TCPFlowDirection]*side{
			true:  newSide(),
			false: newSide(),
		},
		messageQueue: make(chan message),
	}
	f.counter += 1
	f.wrap.WaitGroup().Add(1)
	go s.loop(f.wrap.Context(), f.wrap.WaitGroup())
	return &s
}

type message struct {
	dir     reassembly.TCPFlowDirection
	start   bool
	stop    bool
	skip    int
	payload []byte
}

type streamWrapper struct {
	sid          int
	wrap         HttpStream
	fsm          *reassembly.TCPSimpleFSM
	opt          reassembly.TCPOptionCheck
	messageQueue chan message
	sides        map[reassembly.TCPFlowDirection]*side
}

type side struct {
	buffer       []byte
	requestQueue [][]byte
}

func newSide() *side {
	return &side{
		buffer:       nil,
		requestQueue: make([][]byte, 0, 8),
	}
}

func (s *streamWrapper) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	slog.Debug("stream accept", "stream", s.sid, "tcp", tcp.TransportFlow(), "dir", dir)
	return true
}

func (s *streamWrapper) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	l, _ := sg.Lengths()
	if l == 0 {
		// http will always have content
		return
	}

	dir, start, stop, skip := sg.Info()
	slog.Debug("stream reassembled", "stream", s.sid, "len", l)
	if skip > 0 {
		slog.Warn("dropped bytes", "skip", skip)
	}

	payload := sg.Fetch(l)
	msg := message{
		dir:     dir,
		start:   start,
		stop:    stop,
		skip:    skip,
		payload: payload,
	}

	s.messageQueue <- msg
}

func (s *streamWrapper) ReassemblyComplete(ac reassembly.AssemblerContext) bool {
	slog.Debug("stream reassembly complete", "stream", s.sid)
	//close(s.messageQueue)
	return false
}

func (s *streamWrapper) loop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("stream starting", "stream", s.sid)
	defer slog.Info("stream done", "stream", s.sid)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.messageQueue:
			s.reassembledMessage(msg)
		}
	}
}

func (s *streamWrapper) reassembledMessage(msg message) {
	lhs := s.sides[msg.dir]
	rhs := s.sides[!msg.dir]

	if len(lhs.buffer) >= 0 {
		lhs.buffer = append(lhs.buffer, msg.payload...)
	} else {
		// no idea what the cap will be on this
		lhs.buffer = msg.payload
	}

	// try request
	req, reqErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(lhs.buffer)))
	if reqErr == nil {
		// affirmative on the request path
		_, bodyErr := io.ReadAll(req.Body)
		defer req.Body.Close()

		if errors.Is(bodyErr, io.ErrUnexpectedEOF) {
			// still need more out of the stream
			slog.Debug("waiting for more request data", "have", len(lhs.buffer))
			return
		}

		lhs.requestQueue = append(lhs.requestQueue, lhs.buffer)
		lhs.buffer = make([]byte, 0, 512)
		return
	}

	var rhsReq *http.Request
	if len(rhs.requestQueue) > 0 {
		r, rhsReqErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(rhs.requestQueue[0])))
		if rhsReqErr != nil {
			panic("shouldn't fail because we already checked its a request")
		}
		rhsReq = r
	}

	// try response
	res, resErr := http.ReadResponse(bufio.NewReader(bytes.NewReader(lhs.buffer)), rhsReq)
	if resErr == nil {
		// affirmative on response path
		_, bodyErr := io.ReadAll(res.Body)
		defer res.Body.Close()

		if errors.Is(bodyErr, io.ErrUnexpectedEOF) {
			// still need more out of the stream
			slog.Debug("waiting for more response data", "have", len(lhs.buffer))
			return
		}

		if rhsReq != nil {
			slog.Debug("handled rr")
			s.wrap.ReassembledRequestResponse(rhs.requestQueue[0], lhs.buffer)
			lhs.buffer = nil
			rhs.requestQueue = rhs.requestQueue[1:]
		} else {
			slog.Debug("dropped rr")
			lhs.buffer = nil
		}

		return
	}
}
