package torlab

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestStartRunsChutneyAndDiscoversClientEndpoints(t *testing.T) {
	dir := t.TempDir()
	runner := &recordingRunner{networkJSON: testNetworkJSON(t)}

	lab, err := Start(context.Background(), Options{
		DataDir:       dir,
		ChutneyPath:   "/chutney-src",
		BootstrapTime: time.Second,
		Command:       runner,
	})
	if err != nil {
		t.Fatalf("start lab: %v", err)
	}
	defer lab.Close()

	if lab.SOCKSAddress() != "127.0.0.1:9007" {
		t.Fatalf("socks address = %s", lab.SOCKSAddress())
	}
	if lab.ControlAddress() != "127.0.0.1:8007" {
		t.Fatalf("control address = %s", lab.ControlAddress())
	}
	want := [][]string{
		{"/chutney-src", "/chutney-src/chutney", "--data-dir", dir, "init", "--net", "hs-v3-min"},
		{"/chutney-src", "/chutney-src/chutney", "--data-dir", dir, "bootstrap"},
	}
	if !reflect.DeepEqual(runner.calls[:2], want) {
		t.Fatalf("calls = %#v, want prefix %#v", runner.calls, want)
	}
}

func TestFirstIPv4EndpointIgnoresIPv6(t *testing.T) {
	got := firstIPv4Endpoint([]string{"[::1]:9007", "127.0.0.1:9007"})
	if got != "127.0.0.1:9007" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestExposeMapsLocalListenerToOnion(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	controlAddr, done := startControlServer(t, func(t *testing.T, rw *bufio.ReadWriter) {
		expectCommand(t, rw, "PROTOCOLINFO 1")
		writeReply(t, rw, "250-AUTH METHODS=NULL", "250 OK")
		expectCommand(t, rw, "AUTHENTICATE")
		writeReply(t, rw, "250 OK")
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatalf("split listener addr: %v", err)
		}
		expectCommand(t, rw, "ADD_ONION NEW:ED25519-V3 Flags=DiscardPK Port=8333,127.0.0.1:"+port)
		writeReply(t, rw, "250-ServiceID=peeronion", "250 OK")
	})
	defer done()

	lab := &Lab{control: controlAddr}
	service, err := lab.Expose(context.Background(), 1, listener.Addr().String())
	if err != nil {
		t.Fatalf("expose: %v", err)
	}
	if service.Address != "peeronion.onion:8333" {
		t.Fatalf("service address = %s", service.Address)
	}
}

type recordingRunner struct {
	calls       [][]string
	networkJSON []byte
}

func (r *recordingRunner) Run(_ context.Context, dir string, _ []string, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{dir, name}, args...))
	dataDir := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--data-dir" {
			dataDir = args[i+1]
			break
		}
	}
	if dataDir != "" && args[len(args)-1] == "bootstrap" {
		path := filepath.Join(dataDir, "nodes")
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "network.json"), r.networkJSON, 0o644)
	}
	return nil
}

func testNetworkJSON(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(chutneyNetwork{Nodes: []chutneyNode{{
		Nick:                 "test007c",
		IsClient:             true,
		SOCKSPortEndpoints:   []string{"[::1]:9007", "127.0.0.1:9007"},
		ControlPortEndpoints: []string{"[::1]:8007", "127.0.0.1:8007"},
	}}})
	if err != nil {
		t.Fatalf("marshal network json: %v", err)
	}
	return data
}
