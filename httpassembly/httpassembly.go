package httpassembly

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
)

type HttpAssembler struct {
	pool      *reassembly.StreamPool
	assembler *reassembly.Assembler
	factory   *factoryWrapper
	Log       *slog.Logger
}

func NewAssembler(factory HttpStreamFactory) *HttpAssembler {
	f := &factoryWrapper{
		wrap:         factory,
		counter:      0,
		messageQueue: make(chan message, 128),
		Log:          slog.Default(),
	}
	p := reassembly.NewStreamPool(f)
	a := reassembly.NewAssembler(p)

	// a.MaxBufferedPagesPerConnection = 500
	// a.MaxBufferedPagesTotal = 100000

	return &HttpAssembler{
		pool:      p,
		assembler: a,
		factory:   f,
		Log:       slog.Default(),
	}
}

func (a *HttpAssembler) ReassembleJob(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	a.Log.Debug("reassemble job start")
	defer a.Log.Debug("reassemble job done")

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-a.factory.messageQueue:
			msg.reassemble()
		}
	}
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

func (a *HttpAssembler) FlushCloseOlderThan(t time.Time) {
	a.assembler.FlushCloseOlderThan(t)
}

type HttpStreamFactory interface {
	New() HttpStream
}

type HttpStream interface {
	ReassembledRequestResponse(req []byte, res []byte, duration float64)
}

type factoryWrapper struct {
	wrap         HttpStreamFactory
	counter      int
	messageQueue chan message
	Log          *slog.Logger
}

func (f *factoryWrapper) New(netFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	w := f.wrap.New()

	fsmOptions := reassembly.TCPSimpleFSMOptions{SupportMissingEstablishment: false}
	sid := f.counter
	s := streamWrapper{
		sid:  sid,
		Log:  slog.Default().With("streamID", sid),
		wrap: w,
		fsm:  reassembly.NewTCPSimpleFSM(fsmOptions),
		opt:  reassembly.NewTCPOptionCheck(),
		sides: map[reassembly.TCPFlowDirection]*side{
			true:  newSide(),
			false: newSide(),
		},
		messageQueue: f.messageQueue,
	}
	f.counter += 1
	return &s
}

type message struct {
	s       *streamWrapper
	dir     reassembly.TCPFlowDirection
	start   bool
	stop    bool
	skip    int
	payload []byte
	when    int64
}

type streamWrapper struct {
	sid          int
	Log          *slog.Logger
	wrap         HttpStream
	fsm          *reassembly.TCPSimpleFSM
	opt          reassembly.TCPOptionCheck
	messageQueue chan message
	sides        map[reassembly.TCPFlowDirection]*side
}

type side struct {
	buffer             []byte
	bufferStarts       int64
	bufferEnds         int64
	requestQueue       [][]byte
	requestStartsQueue []int64
}

func newSide() *side {
	return &side{
		buffer:             nil,
		requestQueue:       make([][]byte, 0, 8),
		requestStartsQueue: make([]int64, 0, 8),
	}
}

func (s *streamWrapper) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	//s.Log.Debug("stream accept", "tcp", tcp.TransportFlow(), "dir", dir)
	return true
}

func (s *streamWrapper) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	l, _ := sg.Lengths()
	if l == 0 {
		// http will always have content
		return
	}

	dir, start, stop, skip := sg.Info()
	if skip > 0 {
		s.Log.Warn("dropped bytes", "skip", skip)
	}

	payload := sg.Fetch(l)
	msg := message{
		s:       s,
		dir:     dir,
		start:   start,
		stop:    stop,
		skip:    skip,
		payload: payload,
		when:    ac.GetCaptureInfo().Timestamp.UnixMilli(),
	}

	s.messageQueue <- msg

	//s.Log.Debug("stream reassembled sg", "len", l)
}

func (s *streamWrapper) ReassemblyComplete(ac reassembly.AssemblerContext) bool {
	s.Log.Debug("stream reassembly complete")
	//close(s.messageQueue)
	return false
}

func (msg *message) reassemble() {
	s := msg.s
	lhs := s.sides[msg.dir]
	rhs := s.sides[!msg.dir]

	if len(lhs.buffer) > 0 {
		lhs.buffer = append(lhs.buffer, msg.payload...)
		lhs.bufferEnds = msg.when
	} else {
		// no idea what the cap will be on this
		lhs.buffer = msg.payload
		lhs.bufferStarts = msg.when
		lhs.bufferEnds = msg.when
	}

	// try request
	req, reqErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(lhs.buffer)))
	if reqErr == nil {
		// affirmative on the request path
		_, bodyErr := io.Copy(io.Discard, req.Body)
		defer req.Body.Close()

		if errors.Is(bodyErr, io.ErrUnexpectedEOF) {
			// still need more out of the stream
			//s.Log.Debug("waiting for more request data", "have", len(lhs.buffer))
			return
		}

		lhs.requestQueue = append(lhs.requestQueue, lhs.buffer)
		lhs.requestStartsQueue = append(lhs.requestStartsQueue, lhs.bufferStarts)
		lhs.buffer = make([]byte, 0, 512)
		lhs.bufferStarts = 0
		lhs.bufferEnds = 0
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
		_, bodyErr := io.Copy(io.Discard, res.Body)
		defer res.Body.Close()

		if errors.Is(bodyErr, io.ErrUnexpectedEOF) {
			// still need more out of the stream
			//s.Log.Debug("waiting for more response data", "have", len(lhs.buffer))
			return
		}

		if rhsReq != nil {
			//s.Log.Debug("handled rr")

			// should be in seconds
			duration := float64(lhs.bufferEnds-rhs.requestStartsQueue[0]) / 1000.0
			//s.Log.Debug("duration", "v", duration, "a", lhs.bufferEnds, "b", rhs.requestStartsQueue[0])

			s.wrap.ReassembledRequestResponse(rhs.requestQueue[0], lhs.buffer, duration)
			lhs.buffer = nil
			lhs.bufferStarts = 0
			lhs.bufferEnds = 0
			rhs.requestQueue = rhs.requestQueue[1:]
			rhs.requestStartsQueue = rhs.requestStartsQueue[1:]
		} else {
			//s.Log.Debug("dropped rr")
			lhs.buffer = nil
			lhs.bufferStarts = 0
			lhs.bufferEnds = 0
		}

		return
	}
}
