package peerlab

import (
	"net"
	"testing"

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
