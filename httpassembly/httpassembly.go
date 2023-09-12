package httpassembly

import (
	"bufio"
	"bytes"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
	"log"
	"net/http"
)

type HttpAssembler struct {
	pool      *reassembly.StreamPool
	assembler *reassembly.Assembler
}

func NewAssembler(factory HttpStreamFactory) *HttpAssembler {
	p := reassembly.NewStreamPool(&factoryWrapper{wrap: factory})
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

type HttpStreamFactory interface {
	New() HttpStream
}

type HttpStream interface {
	ReassembledRequestResponse(req *http.Request, res *http.Response)
}

type factoryWrapper struct {
	wrap HttpStreamFactory
}

func (f *factoryWrapper) New(netFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	w := f.wrap.New()
	s := streamWrapper{
		wrap:         w,
		requestQueue: make([]*http.Request, 0, 8),
	}
	return &s
}

type streamWrapper struct {
	wrap         HttpStream
	requestQueue []*http.Request
}

func (s *streamWrapper) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	// when would we reject?
	return true
}

func (s *streamWrapper) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	// TODO the stream is stateful so we should handle bizarre transitions or else risk
	//  being stuck in a bad state
	// TODO monitor data loss

	//d, b, e, k := sg.Info()
	//log.Printf("reassembling (%s, %t, %t, %d) [req %t %d] [res %t %d]", d, b, e, k, s.req == nil, len(s.req), s.res == nil, len(s.res))

	l, _ := sg.Lengths()
	if l == 0 {
		// http will always have content
		return
	}

	payload := sg.Fetch(l)

	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(payload)))
	if err == nil {
		s.requestQueue = append(s.requestQueue, r)
		return
	}

	if len(s.requestQueue) == 0 {
		// there is no request to pair any response with, discard
		return
	}
	r = s.requestQueue[0]
	w, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(payload)), r)
	if err != nil {
		log.Printf("Not a request or a response? %s\n", payload)
		return
	}
	s.requestQueue = s.requestQueue[1:]

	s.wrap.ReassembledRequestResponse(r, w)
}

func (s *streamWrapper) ReassemblyComplete(ac reassembly.AssemblerContext) bool {
	return true
}
