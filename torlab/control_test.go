package torlab

import (
	"bufio"
	"context"
	"encoding/hex"
	"net"
	"os"
	"strings"
	"testing"
)

func TestControlClientCreatesOnionService(t *testing.T) {
	cookie := []byte("test-cookie")
	cookieFile := writeCookie(t, cookie)
	addr, done := startControlServer(t, func(t *testing.T, rw *bufio.ReadWriter) {
		expectCommand(t, rw, "PROTOCOLINFO 1")
		writeReply(t, rw,
			"250-PROTOCOLINFO 1",
			`250-AUTH METHODS=COOKIE COOKIEFILE="`+cookieFile+`"`,
			"250 OK",
		)
		expectCommand(t, rw, "AUTHENTICATE "+hex.EncodeToString(cookie))
		writeReply(t, rw, "250 OK")
		expectCommand(t, rw, "ADD_ONION NEW:ED25519-V3 Flags=DiscardPK Port=8333,127.0.0.1:18444")
		writeReply(t, rw, "250-ServiceID=abcdefghijklmnop", "250 OK")
	})

	service, err := (ControlClient{Address: addr}).AddOnion(context.Background(), 8333, "127.0.0.1:18444")
	if err != nil {
		t.Fatalf("add onion: %v", err)
	}
	if service.Address != "abcdefghijklmnop.onion:8333" || service.Identity != "abcdefghijklmnop.onion" {
		t.Fatalf("service = %+v", service)
	}
	done()
}

func TestControlClientDeletesOnionService(t *testing.T) {
	addr, done := startControlServer(t, func(t *testing.T, rw *bufio.ReadWriter) {
		expectCommand(t, rw, "PROTOCOLINFO 1")
		writeReply(t, rw, "250-AUTH METHODS=NULL", "250 OK")
		expectCommand(t, rw, "AUTHENTICATE")
		writeReply(t, rw, "250 OK")
		expectCommand(t, rw, "DEL_ONION abcdef")
		writeReply(t, rw, "250 OK")
	})

	if err := (ControlClient{Address: addr}).DelOnion(context.Background(), "abcdef"); err != nil {
		t.Fatalf("delete onion: %v", err)
	}
	done()
}

func TestParseControlValue(t *testing.T) {
	line := `AUTH METHODS=COOKIE COOKIEFILE="/tmp/control auth cookie"`
	if got := parseControlValue(line, "COOKIEFILE"); got != "/tmp/control" {
		t.Fatalf("parser should remain field based for Tor paths without spaces, got %q", got)
	}
	line = `AUTH METHODS=COOKIE COOKIEFILE="/tmp/control_auth_cookie"`
	if got := parseControlValue(line, "COOKIEFILE"); got != "/tmp/control_auth_cookie" {
		t.Fatalf("cookie file = %q", got)
	}
}

func startControlServer(t *testing.T, handler func(*testing.T, *bufio.ReadWriter)) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		handler(t, rw)
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func expectCommand(t *testing.T, rw *bufio.ReadWriter, want string) {
	t.Helper()
	line, err := rw.ReadString('\n')
	if err != nil {
		t.Fatalf("read command: %v", err)
	}
	got := strings.TrimRight(line, "\r\n")
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func writeReply(t *testing.T, rw *bufio.ReadWriter, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if _, err := rw.WriteString(line + "\r\n"); err != nil {
			t.Fatalf("write reply: %v", err)
		}
	}
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush reply: %v", err)
	}
}

func writeCookie(t *testing.T, cookie []byte) string {
	t.Helper()
	path := t.TempDir() + "/control_auth_cookie"
	if err := os.WriteFile(path, cookie, 0o600); err != nil {
		t.Fatalf("write cookie: %v", err)
	}
	return path
}
