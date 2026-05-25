// Package torlab starts a private Tor network and exposes peerlab listeners
// as v3 onion services.
package torlab

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// OnionService is one peerlab listener exposed through Tor.
type OnionService struct {
	ServiceID string
	Address   string
	Identity  string
	VirtPort  int
	Target    string
}

// ControlClient speaks the small subset of the Tor control protocol that the
// harness needs: authenticate, create an ephemeral onion service, and remove
// it during cleanup.
type ControlClient struct {
	Address string
	Timeout time.Duration
}

// AddOnion creates an ephemeral v3 onion service for target.
func (c ControlClient) AddOnion(ctx context.Context, virtPort int, target string) (OnionService, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return OnionService{}, err
	}
	defer conn.Close()

	control := newControlConn(conn)
	if err := control.authenticate(); err != nil {
		return OnionService{}, err
	}
	lines, err := control.command(
		"ADD_ONION NEW:ED25519-V3 Flags=DiscardPK Port=%d,%s",
		virtPort,
		target,
	)
	if err != nil {
		return OnionService{}, err
	}
	serviceID := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "ServiceID=") {
			serviceID = strings.TrimPrefix(line, "ServiceID=")
			break
		}
	}
	if serviceID == "" {
		return OnionService{}, fmt.Errorf("ADD_ONION response did not include ServiceID: %q", lines)
	}
	return OnionService{
		ServiceID: serviceID,
		Address:   net.JoinHostPort(serviceID+".onion", strconv.Itoa(virtPort)),
		Identity:  serviceID + ".onion",
		VirtPort:  virtPort,
		Target:    target,
	}, nil
}

// DelOnion removes an ephemeral onion service created by AddOnion.
func (c ControlClient) DelOnion(ctx context.Context, serviceID string) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	control := newControlConn(conn)
	if err := control.authenticate(); err != nil {
		return err
	}
	_, err = control.command("DEL_ONION %s", serviceID)
	return err
}

func (c ControlClient) dial(ctx context.Context) (net.Conn, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return nil, fmt.Errorf("dial tor control %s: %w", c.Address, err)
	}
	return conn, nil
}

type controlConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func newControlConn(conn net.Conn) *controlConn {
	return &controlConn{conn: conn, r: bufio.NewReader(conn)}
}

func (c *controlConn) authenticate() error {
	lines, err := c.command("PROTOCOLINFO 1")
	if err != nil {
		return err
	}
	cookieFile := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "AUTH ") {
			cookieFile = parseControlValue(line, "COOKIEFILE")
			break
		}
	}
	if cookieFile == "" {
		_, err := c.command("AUTHENTICATE")
		return err
	}
	cookie, err := os.ReadFile(cookieFile)
	if err != nil {
		return fmt.Errorf("read tor control cookie: %w", err)
	}
	_, err = c.command("AUTHENTICATE %s", hex.EncodeToString(cookie))
	return err
}

func (c *controlConn) command(format string, args ...any) ([]string, error) {
	if _, err := fmt.Fprintf(c.conn, format+"\r\n", args...); err != nil {
		return nil, fmt.Errorf("write tor control command: %w", err)
	}
	return c.readReply()
}

func (c *controlConn) readReply() ([]string, error) {
	var lines []string
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read tor control reply: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 3 {
			return nil, fmt.Errorf("short tor control reply line %q", line)
		}
		code := line[:3]
		rest := ""
		if len(line) > 4 {
			rest = line[4:]
		}
		if code != "250" {
			return lines, fmt.Errorf("tor control returned %s: %s", code, rest)
		}
		lines = append(lines, rest)
		if len(line) == 3 || line[3] == ' ' {
			return lines, nil
		}
	}
}

func parseControlValue(line, key string) string {
	prefix := key + "="
	for _, field := range strings.Fields(line) {
		if !strings.HasPrefix(field, prefix) {
			continue
		}
		value := strings.TrimPrefix(field, prefix)
		value = strings.Trim(value, `"`)
		return value
	}
	return ""
}
