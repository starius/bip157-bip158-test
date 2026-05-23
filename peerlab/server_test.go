package peerlab

import (
	"net"
	"testing"
	"time"

	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/btcsuite/btcd/wire"
)

func TestServerServesHeadersAndFilters(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := NewServer(fixture)
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close()

	sendVersion(t, conn, fixture)
	readUntilVerAck(t, conn, fixture)

	getHeaders := wire.NewMsgGetHeaders()
	genesis := fixture.Blocks[0].Block.BlockHash()
	_ = getHeaders.AddBlockLocatorHash(&genesis)
	if err := wire.WriteMessage(conn, getHeaders, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getheaders: %v", err)
	}
	headers := readMessageOf[*wire.MsgHeaders](t, conn, fixture)
	if len(headers.Headers) != 2 {
		t.Fatalf("expected two headers after genesis, got %d", len(headers.Headers))
	}

	stop := fixture.Blocks[2].Block.BlockHash()
	getCFHeaders := wire.NewMsgGetCFHeaders(wire.GCSFilterRegular, 1, &stop)
	if err := wire.WriteMessage(conn, getCFHeaders, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getcfheaders: %v", err)
	}
	cfHeaders := readMessageOf[*wire.MsgCFHeaders](t, conn, fixture)
	if len(cfHeaders.FilterHashes) != 2 {
		t.Fatalf("expected two filter hashes, got %d", len(cfHeaders.FilterHashes))
	}

	getFilters := wire.NewMsgGetCFilters(wire.GCSFilterRegular, 1, &stop)
	if err := wire.WriteMessage(conn, getFilters, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getcfilters: %v", err)
	}
	for i := 0; i < 2; i++ {
		filter := readMessageOf[*wire.MsgCFilter](t, conn, fixture)
		if len(filter.Data) == 0 {
			t.Fatalf("filter %d was empty", i)
		}
	}
}

func TestServerCanCorruptCFHeaders(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := NewServer(fixture, WithBehavior(Behavior{
		CorruptCFHeaders: map[uint32]bool{2: true},
	}))
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	conn := dialAndHandshake(t, server, fixture)
	defer conn.Close()

	stop := fixture.Blocks[2].Block.BlockHash()
	getCFHeaders := wire.NewMsgGetCFHeaders(wire.GCSFilterRegular, 1, &stop)
	if err := wire.WriteMessage(conn, getCFHeaders, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getcfheaders: %v", err)
	}
	cfHeaders := readMessageOf[*wire.MsgCFHeaders](t, conn, fixture)
	if *cfHeaders.FilterHashes[0] != fixture.Blocks[1].Filter.FilterHash {
		t.Fatalf("height 1 filter hash should stay honest")
	}
	if *cfHeaders.FilterHashes[1] == fixture.Blocks[2].Filter.FilterHash {
		t.Fatalf("height 2 filter hash was not corrupted")
	}
}

func TestServerCanCorruptCFilter(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := NewServer(fixture, WithBehavior(Behavior{
		CorruptCFilters: map[uint32]bool{2: true},
	}))
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	conn := dialAndHandshake(t, server, fixture)
	defer conn.Close()

	stop := fixture.Blocks[2].Block.BlockHash()
	getFilters := wire.NewMsgGetCFilters(wire.GCSFilterRegular, 2, &stop)
	if err := wire.WriteMessage(conn, getFilters, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getcfilters: %v", err)
	}
	filter := readMessageOf[*wire.MsgCFilter](t, conn, fixture)
	if chainlab.EqualBytes(filter.Data, fixture.Blocks[2].Filter.FilterBytes) {
		t.Fatalf("height 2 filter bytes were not corrupted")
	}
}

func TestServerCanDelayResponses(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := NewServer(fixture, WithBehavior(Behavior{
		DelayByCommand: map[string]time.Duration{"headers": 25 * time.Millisecond},
	}))
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	conn := dialAndHandshake(t, server, fixture)
	defer conn.Close()

	getHeaders := wire.NewMsgGetHeaders()
	started := time.Now()
	if err := wire.WriteMessage(conn, getHeaders, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getheaders: %v", err)
	}
	_ = readMessageOf[*wire.MsgHeaders](t, conn, fixture)
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Fatalf("headers response was not delayed enough: %s", elapsed)
	}
}

func TestServerCanDelayOneResponseThenRecover(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	server := NewServer(fixture, WithBehavior(Behavior{
		DelayOnceByCommand: map[string]time.Duration{"headers": 25 * time.Millisecond},
	}))
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	conn := dialAndHandshake(t, server, fixture)
	defer conn.Close()

	first := timeHeadersResponse(t, conn, fixture)
	second := timeHeadersResponse(t, conn, fixture)
	if first < 20*time.Millisecond {
		t.Fatalf("first headers response was not delayed enough: %s", first)
	}
	if second > 75*time.Millisecond {
		t.Fatalf("second headers response should recover promptly, got %s", second)
	}
}

func TestUnknownWireErrorsAreIgnorable(t *testing.T) {
	err := errString("received unknown message")
	if !isIgnorableWireError(err) {
		t.Fatalf("unknown message should be ignorable")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func timeHeadersResponse(t *testing.T, conn net.Conn, fixture *chainlab.Fixture) time.Duration {
	t.Helper()
	getHeaders := wire.NewMsgGetHeaders()
	started := time.Now()
	if err := wire.WriteMessage(conn, getHeaders, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send getheaders: %v", err)
	}
	_ = readMessageOf[*wire.MsgHeaders](t, conn, fixture)
	return time.Since(started)
}

func dialAndHandshake(t *testing.T, server *Server, fixture *chainlab.Fixture) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	sendVersion(t, conn, fixture)
	readUntilVerAck(t, conn, fixture)
	return conn
}

func sendVersion(t *testing.T, conn net.Conn, fixture *chainlab.Fixture) {
	t.Helper()
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	na := wire.NewNetAddress(addr, wire.SFNodeNetwork|wire.SFNodeWitness|wire.SFNodeCF)
	version := wire.NewMsgVersion(na, na, 1, 0)
	version.Services = wire.SFNodeNetwork | wire.SFNodeWitness | wire.SFNodeCF
	if err := wire.WriteMessage(conn, version, wire.ProtocolVersion, fixture.Params.Net); err != nil {
		t.Fatalf("send version: %v", err)
	}
}

func readUntilVerAck(t *testing.T, conn net.Conn, fixture *chainlab.Fixture) {
	t.Helper()
	for {
		msg, _, err := wire.ReadMessage(conn, wire.ProtocolVersion, fixture.Params.Net)
		if err != nil {
			t.Fatalf("read handshake: %v", err)
		}
		if _, ok := msg.(*wire.MsgVersion); ok {
			if err := wire.WriteMessage(conn, wire.NewMsgVerAck(), wire.ProtocolVersion, fixture.Params.Net); err != nil {
				t.Fatalf("send verack: %v", err)
			}
			continue
		}
		if _, ok := msg.(*wire.MsgVerAck); ok {
			return
		}
	}
}

func readMessageOf[T wire.Message](t *testing.T, conn net.Conn, fixture *chainlab.Fixture) T {
	t.Helper()
	for {
		msg, _, err := wire.ReadMessage(conn, wire.ProtocolVersion, fixture.Params.Net)
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		if typed, ok := msg.(T); ok {
			return typed
		}
	}
}
