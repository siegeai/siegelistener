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

type HttpStreamFactory interface {
	New() HttpStream
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

	//log.Printf("new stream for %s %s", netFlow, tcpFlow)

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
	go s.loop()
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
	requestQueue     [][]byte
	chunks           [][]byte
	chunksAreRequest bool
}

func newSide() *side {
	return &side{
		requestQueue:     make([][]byte, 0, 8),
		chunks:           nil,
		chunksAreRequest: false,
	}
}

func (s *streamWrapper) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	if !s.fsm.CheckState(tcp, dir) {
		log.Printf("rejecting because of fsm")
		return false
	}

	if err := s.opt.Accept(tcp, ci, dir, nextSeq, start); err != nil {
		log.Printf("rejecting because of opts")
		return false
	}

	//log.Printf("packet: %s\n", string(tcp.Payload))
	//log.Println("accepting")
	return true
}

func (s *streamWrapper) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	l, _ := sg.Lengths()
	if l == 0 {
		// http will always have content
		return
	}

	dir, start, stop, skip := sg.Info()
	payload := sg.Fetch(l)

	if skip > 0 {
		log.Printf("Dropped %d bytes", skip)
	}

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
	log.Println("reassembly complete")
	//close(s.messageQueue)
	return false
}

func (s *streamWrapper) loop() {
	//for msg := range s.messageQueue {
	//	s.reassembledMessage(msg)
	//}

	// TODO shutdown somehow. Should pass in a sync.WaitGroup
	for {
		select {
		case msg := <-s.messageQueue:
			s.reassembledMessage(msg)
		}
	}
}

func (s *streamWrapper) reassembledMessage(msg message) {
	//log.Printf("reassembly state: id %d tql %d, tcl %d, fql %d fcl %d\n", s.sid, len(s.sides[true].requestQueue), len(s.sides[true].chunks), len(s.sides[false].requestQueue), len(s.sides[false].chunks))

	if isRequest(msg.payload) {
		s.reassembledRequest(msg)
		return
	}
	if isResponse(msg.payload) {
		s.reassembledResponse(msg)
		return
	}
	if isChunk(msg.payload) {
		s.reassembledChunk(msg)
		return
	}

	log.Printf("Unknown message with payload of len %d: %s...", len(msg.payload), string(msg.payload)[0:min(len(msg.payload), 32)])
}

func (s *streamWrapper) reassembledRequest(msg message) {
	v := s.sides[msg.dir]

	if len(v.chunks) > 0 {
		log.Printf("Expected a chunk but got a request")
		s.chunksDone(&msg)
		//return
	}

	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(msg.payload)))
	if err != nil {
		log.Printf("httpassembly Could not parse request because %s\n%s\n", err, string(msg.payload))
		return
	}

	chunked := false
	for _, enc := range r.TransferEncoding {
		if "chunked" == enc {
			chunked = true
		}
	}

	if chunked {
		if bytes.HasSuffix(msg.payload, []byte("0\r\n\r\n")) {
			// we have all the data so treat it as not chunked
			chunked = false
		} else {
			log.Printf("Chunked request, no end in sight")
		}
	}

	if chunked {
		v.chunksAreRequest = true
		v.chunks = make([][]byte, 0, 8)
		v.chunks = append(v.chunks, msg.payload)

	} else {
		v.requestQueue = append(v.requestQueue, msg.payload)
	}
}

func (s *streamWrapper) reassembledResponse(msg message) {
	v := s.sides[msg.dir]

	if len(v.chunks) > 0 {
		log.Printf("Expected a chunk but got a response")
		s.chunksDone(&msg)
		//return
	}

	o := s.sides[!msg.dir]
	var r *http.Request
	if len(o.requestQueue) > 0 {
		var err error
		r, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(o.requestQueue[0])))
		if err != nil {
			log.Printf("Could not parse request, WTF?")
		}
	}

	w, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(msg.payload)), r)
	if err != nil {
		log.Printf("Could not parse response because %s", err)
		return
	}

	chunked := false
	for _, enc := range w.TransferEncoding {
		if "chunked" == enc {
			chunked = true
		}
	}

	if chunked {
		if bytes.HasSuffix(msg.payload, []byte("0\r\n\r\n")) {
			// we have all the data so treat it as not chunked
			chunked = false
		} else {
			log.Printf("Chunked response, no end in sight")
		}
	}

	if chunked {
		v.chunksAreRequest = false
		v.chunks = make([][]byte, 0, 8)
		v.chunks = append(v.chunks, msg.payload)

	} else {
		if r == nil {
			// discard because we have no request to pair with
			return
		}

		s.wrap.ReassembledRequestResponse(o.requestQueue[0], msg.payload)
		o.requestQueue = o.requestQueue[1:]
	}
}

func (s *streamWrapper) reassembledChunk(msg message) {
	v := s.sides[msg.dir]

	if len(v.chunks) == 0 {
		log.Printf("Expected to follow a chunked message")
		return
	}

	v.chunks = append(v.chunks, msg.payload)

	// If it's not the last chunk we are done for now
	if !bytes.HasSuffix(msg.payload, []byte("0\r\n\r\n")) {
		return
	}

	s.chunksDone(&msg)
}

func (s *streamWrapper) chunksDone(msg *message) {

	v := s.sides[msg.dir]

	full := bytes.Join(v.chunks, []byte{})
	v.chunks = nil

	if v.chunksAreRequest {
		_, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(full)))
		if err != nil {
			log.Printf("Could not parse (chunked) request because %s", err)
			return
		}
		v.requestQueue = append(v.requestQueue, full)

	} else {
		_, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(full)), nil)
		if err != nil {
			log.Printf("Could not parse (chunked) response because %s", err)
			return
		}

		o := s.sides[!msg.dir]
		if len(o.requestQueue) == 0 {
			// discard because we have no request to pair with
			return
		}

		s.wrap.ReassembledRequestResponse(o.requestQueue[0], full)
		o.requestQueue = o.requestQueue[1:]
	}
}

func isRequest(bs []byte) bool {
	switch string(prefix(bs)) {
	case "GET", "HEAD", "POST", "PUT", "DELETE", "TRACE", "CONNECT":
		return true
	}
	return false
}

func isResponse(bs []byte) bool {
	if bytes.HasPrefix(bs, []byte("HTTP/")) {
		return true
	}
	return false
}

func isChunk(bs []byte) bool {
	// Chunks start with a hex number denoting their length
	return isHex(prefix(bs))
}

func prefix(bs []byte) []byte {
	for i, b := range bs {
		// SP CR LF HTAB
		if b == 0x20 || b == 0x0D || b == 0x0A || b == 0x09 {
			return bs[0:i]
		}
	}
	return bs
}

func isHex(bs []byte) bool {
	if len(bs) == 0 {
		return false
	}

	for _, b := range bs {
		// 0-9 -> 48-57
		// A-F -> 65-70
		// a-f -> 97-102
		if !((48 <= b && b <= 57) || (65 <= b && b <= 70) || (97 <= b && b <= 102)) {
			return false
		}
	}

	return true
}
