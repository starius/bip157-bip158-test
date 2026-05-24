// Package peerlab provides a small harness-controlled Bitcoin P2P server.
//
// The server exists because honest bitcoind nodes cannot produce the malformed,
// conflicting, delayed, or equivocated BIP157 responses needed for conformance
// testing. Peerlab starts with honest behavior and records a transcript so each
// scenario can explain exactly what the client asked for and what it received.
package peerlab

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// TranscriptEntry records one Bitcoin P2P message observed by the simulator.
type TranscriptEntry struct {
	At      time.Time
	Peer    string
	Dir     string
	Command string
	Summary string
}

// Behavior describes deterministic faults a scenario wants the peer to inject.
// Height maps use block heights from the fixture, not p2p message indexes.
type Behavior struct {
	CorruptHeaders          map[uint32]bool
	CorruptCFHeaders        map[uint32]bool
	CorruptCFCheckpts       map[uint32]bool
	CorruptCFilters         map[uint32]bool
	CorruptPrevFilterHeader map[uint32]bool
	EmptyCFHeaders          map[uint32]bool
	WrongCFilterBlockHash   map[uint32]bool
	CorruptBlocks           map[uint32]bool
	DelayByCommand          map[string]time.Duration
	DelayOnceByCommand      map[string]time.Duration
	WrongFilterType         map[string]wire.FilterType
}

// Option customizes a peer simulator.
type Option func(*Server)

// WithBehavior makes the peer inject deterministic faults into otherwise
// normal BIP157 responses.
func WithBehavior(behavior Behavior) Option {
	return func(s *Server) {
		s.behavior = cloneBehavior(behavior)
	}
}

// Server is an honest BIP157 regtest peer backed by a chainlab fixture.
type Server struct {
	params   *chaincfg.Params
	fixture  *chainlab.Fixture
	behavior Behavior

	listener net.Listener
	done     chan struct{}

	mu         sync.Mutex
	transcript []TranscriptEntry
}

// NewServer returns a peer simulator for fixture.
func NewServer(fixture *chainlab.Fixture, opts ...Option) *Server {
	server := &Server{
		params:  fixture.Params,
		fixture: fixture,
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(server)
	}
	return server
}

// Start begins listening on addr. Use "127.0.0.1:0" to allocate a free port.
func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln
	go s.acceptLoop()
	return nil
}

// Stop closes the listener and all future accepts. Existing connections exit
// when their next read or write fails.
func (s *Server) Stop() error {
	close(s.done)
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Addr returns the listener address.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Transcript returns a stable copy of the P2P transcript.
func (s *Server) Transcript() []TranscriptEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TranscriptEntry, len(s.transcript))
	copy(out, s.transcript)
	return out
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	peer := conn.RemoteAddr().String()

	for {
		msg, _, err := wire.ReadMessage(conn, wire.ProtocolVersion, s.params.Net)
		if err != nil {
			if isIgnorableWireError(err) {
				s.record(peer, "in", "unknown", err.Error())
				continue
			}
			s.record(peer, "in", "disconnect", err.Error())
			return
		}
		s.record(peer, "in", msg.Command(), summarize(msg))

		if err := s.handleMessage(conn, peer, msg); err != nil {
			s.record(peer, "out", "error", err.Error())
			return
		}
	}
}

func (s *Server) handleMessage(conn net.Conn, peer string, msg wire.Message) error {
	switch m := msg.(type) {
	case *wire.MsgVersion:
		local, remote := netAddresses(conn)
		version := wire.NewMsgVersion(local, remote, uint64(time.Now().UnixNano()), int32(len(s.fixture.Blocks)-1))
		version.Services = wire.SFNodeNetwork | wire.SFNodeWitness | wire.SFNodeCF
		if err := s.write(conn, peer, version); err != nil {
			return err
		}
		return s.write(conn, peer, wire.NewMsgVerAck())

	case *wire.MsgVerAck:
		return nil

	case *wire.MsgPing:
		return s.write(conn, peer, wire.NewMsgPong(m.Nonce))

	case *wire.MsgGetHeaders:
		return s.write(conn, peer, s.headersResponse(m))

	case *wire.MsgGetCFHeaders:
		resp, err := s.cfHeadersResponse(m)
		if err != nil {
			return err
		}
		return s.write(conn, peer, resp)

	case *wire.MsgGetCFilters:
		responses, err := s.cfiltersResponse(m)
		if err != nil {
			return err
		}
		for _, resp := range responses {
			if err := s.write(conn, peer, resp); err != nil {
				return err
			}
		}
		return nil

	case *wire.MsgGetCFCheckpt:
		return s.write(conn, peer, s.cfCheckptResponse(m))

	case *wire.MsgGetData:
		for _, inv := range m.InvList {
			if inv.Type != wire.InvTypeBlock && inv.Type != wire.InvTypeWitnessBlock {
				continue
			}
			block := s.blockByHash(inv.Hash)
			if block == nil {
				continue
			}
			msg := block.Block
			if s.behavior.CorruptBlocks[block.Height] {
				msg = corruptBlock(block.Block)
			}
			if err := s.write(conn, peer, msg); err != nil {
				return err
			}
		}
		return nil

	case *wire.MsgSendHeaders, *wire.MsgSendAddrV2, *wire.MsgFeeFilter:
		return nil
	default:
		return nil
	}
}

func (s *Server) write(conn net.Conn, peer string, msg wire.Message) error {
	if delay := s.delayFor(msg.Command()); delay > 0 {
		time.Sleep(delay)
	}
	s.record(peer, "out", msg.Command(), summarize(msg))
	return wire.WriteMessage(conn, msg, wire.ProtocolVersion, s.params.Net)
}

func (s *Server) headersResponse(req *wire.MsgGetHeaders) *wire.MsgHeaders {
	start := s.locatorStart(req.BlockLocatorHashes)
	resp := wire.NewMsgHeaders()
	for i := start; i < len(s.fixture.Blocks); i++ {
		header := s.fixture.Blocks[i].Block.Header
		if s.behavior.CorruptHeaders[uint32(i)] {
			header.PrevBlock = corruptHash(header.PrevBlock)
		}
		_ = resp.AddBlockHeader(&header)
		if req.HashStop == s.fixture.Blocks[i].Block.BlockHash() {
			break
		}
		if len(resp.Headers) == wire.MaxBlockHeadersPerMsg {
			break
		}
	}
	return resp
}

func (s *Server) cfHeadersResponse(req *wire.MsgGetCFHeaders) (*wire.MsgCFHeaders, error) {
	stopHeight := s.heightOf(req.StopHash)
	if stopHeight < 0 {
		return nil, fmt.Errorf("unknown cfheaders stop hash %s", req.StopHash)
	}
	if int(req.StartHeight) > stopHeight {
		return nil, fmt.Errorf("cfheaders start height %d after stop height %d", req.StartHeight, stopHeight)
	}

	resp := wire.NewMsgCFHeaders()
	resp.FilterType = s.responseFilterType("cfheaders", req.FilterType)
	resp.StopHash = req.StopHash
	if req.StartHeight > 0 {
		resp.PrevFilterHeader = s.fixture.Blocks[req.StartHeight-1].Filter.FilterHeader
	}
	if s.behavior.CorruptPrevFilterHeader[req.StartHeight] {
		resp.PrevFilterHeader = corruptHash(resp.PrevFilterHeader)
	}
	if s.behavior.EmptyCFHeaders[req.StartHeight] {
		return resp, nil
	}
	for h := int(req.StartHeight); h <= stopHeight; h++ {
		hash := s.fixture.Blocks[h].Filter.FilterHash
		if s.behavior.CorruptCFHeaders[uint32(h)] {
			hash = corruptHash(hash)
		}
		_ = resp.AddCFHash(&hash)
		if len(resp.FilterHashes) == wire.MaxCFHeadersPerMsg {
			break
		}
	}
	return resp, nil
}

func (s *Server) cfiltersResponse(req *wire.MsgGetCFilters) ([]wire.Message, error) {
	stopHeight := s.heightOf(req.StopHash)
	if stopHeight < 0 {
		return nil, fmt.Errorf("unknown cfilters stop hash %s", req.StopHash)
	}
	var out []wire.Message
	for h := int(req.StartHeight); h <= stopHeight; h++ {
		block := s.fixture.Blocks[h]
		hash := block.Block.BlockHash()
		if s.behavior.WrongCFilterBlockHash[uint32(h)] {
			hash = corruptHash(hash)
		}
		data := append([]byte(nil), block.Filter.FilterBytes...)
		if s.behavior.CorruptCFilters[uint32(h)] {
			data = corruptBytes(data)
		}
		filterType := s.responseFilterType("cfilter", req.FilterType)
		out = append(out, wire.NewMsgCFilter(filterType, &hash, data))
	}
	return out, nil
}

func (s *Server) cfCheckptResponse(req *wire.MsgGetCFCheckpt) *wire.MsgCFCheckpt {
	filterType := s.responseFilterType("cfcheckpt", req.FilterType)
	stopHeight := s.heightOf(req.StopHash)
	headersCount := 0
	if stopHeight > 0 {
		headersCount = stopHeight / int(wire.CFCheckptInterval)
	}
	resp := wire.NewMsgCFCheckpt(filterType, &req.StopHash, headersCount)
	for h := int(wire.CFCheckptInterval); h <= stopHeight; h += int(wire.CFCheckptInterval) {
		header := s.fixture.Blocks[h].Filter.FilterHeader
		if s.behavior.CorruptCFCheckpts[uint32(h)] {
			header = corruptHash(header)
		}
		_ = resp.AddCFHeader(&header)
	}
	return resp
}

func (s *Server) locatorStart(locators []*chainhash.Hash) int {
	if len(locators) == 0 {
		return 0
	}
	for _, locator := range locators {
		for h, block := range s.fixture.Blocks {
			hash := block.Block.BlockHash()
			if hash == *locator {
				return h + 1
			}
		}
	}
	return 0
}

func (s *Server) heightOf(hash chainhash.Hash) int {
	for h, block := range s.fixture.Blocks {
		if block.Block.BlockHash() == hash {
			return h
		}
	}
	return -1
}

func (s *Server) blockByHash(hash chainhash.Hash) *chainlab.BlockFixture {
	for i := range s.fixture.Blocks {
		block := &s.fixture.Blocks[i]
		if block.Block.BlockHash() == hash {
			return block
		}
	}
	return nil
}

func (s *Server) record(peer, dir, command, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transcript = append(s.transcript, TranscriptEntry{
		At:      time.Now(),
		Peer:    peer,
		Dir:     dir,
		Command: command,
		Summary: summary,
	})
}

func netAddresses(conn net.Conn) (*wire.NetAddress, *wire.NetAddress) {
	local := tcpAddr(conn.LocalAddr())
	remote := tcpAddr(conn.RemoteAddr())
	return wire.NewNetAddress(local, wire.SFNodeNetwork|wire.SFNodeWitness|wire.SFNodeCF),
		wire.NewNetAddress(remote, wire.SFNodeNetwork|wire.SFNodeWitness|wire.SFNodeCF)
}

func tcpAddr(addr net.Addr) *net.TCPAddr {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		return tcp
	}
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func summarize(msg wire.Message) string {
	switch m := msg.(type) {
	case *wire.MsgHeaders:
		return fmt.Sprintf("%d headers", len(m.Headers))
	case *wire.MsgCFHeaders:
		return fmt.Sprintf("%d filter hashes stop=%s", len(m.FilterHashes), m.StopHash)
	case *wire.MsgCFilter:
		return fmt.Sprintf("block=%s bytes=%d", m.BlockHash, len(m.Data))
	case *wire.MsgGetCFHeaders:
		return fmt.Sprintf("start=%d stop=%s", m.StartHeight, m.StopHash)
	case *wire.MsgGetCFilters:
		return fmt.Sprintf("start=%d stop=%s", m.StartHeight, m.StopHash)
	case *wire.MsgGetCFCheckpt:
		return fmt.Sprintf("stop=%s", m.StopHash)
	case *wire.MsgGetHeaders:
		return fmt.Sprintf("%d locators stop=%s", len(m.BlockLocatorHashes), m.HashStop)
	default:
		return ""
	}
}

func isIgnorableWireError(err error) bool {
	return strings.Contains(err.Error(), "unknown message")
}

func corruptHash(hash chainhash.Hash) chainhash.Hash {
	hash[0] ^= 0x01
	return hash
}

func corruptBytes(data []byte) []byte {
	if len(data) == 0 {
		return []byte{0xff}
	}
	out := append([]byte(nil), data...)
	out[len(out)-1] ^= 0x01
	return out
}

func corruptBlock(block *wire.MsgBlock) *wire.MsgBlock {
	out := block.Copy()
	for _, tx := range out.Transactions {
		if len(tx.TxOut) == 0 {
			continue
		}
		tx.TxOut[0].PkScript = append(append([]byte(nil), tx.TxOut[0].PkScript...), 0x51)
		return out
	}
	out.Header.Nonce ^= 0x01
	return out
}

func cloneBehavior(behavior Behavior) Behavior {
	return Behavior{
		CorruptHeaders:          cloneBoolMap(behavior.CorruptHeaders),
		CorruptCFHeaders:        cloneBoolMap(behavior.CorruptCFHeaders),
		CorruptCFCheckpts:       cloneBoolMap(behavior.CorruptCFCheckpts),
		CorruptCFilters:         cloneBoolMap(behavior.CorruptCFilters),
		CorruptPrevFilterHeader: cloneBoolMap(behavior.CorruptPrevFilterHeader),
		EmptyCFHeaders:          cloneBoolMap(behavior.EmptyCFHeaders),
		WrongCFilterBlockHash:   cloneBoolMap(behavior.WrongCFilterBlockHash),
		CorruptBlocks:           cloneBoolMap(behavior.CorruptBlocks),
		DelayByCommand:          cloneDurationMap(behavior.DelayByCommand),
		DelayOnceByCommand:      cloneDurationMap(behavior.DelayOnceByCommand),
		WrongFilterType:         cloneFilterTypeMap(behavior.WrongFilterType),
	}
}

func (s *Server) delayFor(command string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if delay := s.behavior.DelayOnceByCommand[command]; delay > 0 {
		delete(s.behavior.DelayOnceByCommand, command)
		return delay
	}
	return s.behavior.DelayByCommand[command]
}

func cloneBoolMap(in map[uint32]bool) map[uint32]bool {
	out := make(map[uint32]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDurationMap(in map[string]time.Duration) map[string]time.Duration {
	out := make(map[string]time.Duration, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneFilterTypeMap(in map[string]wire.FilterType) map[string]wire.FilterType {
	out := make(map[string]wire.FilterType, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *Server) responseFilterType(command string, requested wire.FilterType) wire.FilterType {
	if filterType, ok := s.behavior.WrongFilterType[command]; ok {
		return filterType
	}
	return requested
}

// WaitForMessage blocks until the transcript contains command or ctx expires.
// Tests and scenarios use this instead of sleeps so failures are deterministic.
func (s *Server) WaitForMessage(ctx context.Context, command string) bool {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		for _, entry := range s.Transcript() {
			if entry.Command == command {
				return true
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
