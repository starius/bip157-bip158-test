package torlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultNetwork = "hs-v3-min"

// Options configures one private Chutney-backed Tor lab.
type Options struct {
	DataDir       string
	ChutneyPath   string
	Network       string
	BootstrapTime time.Duration
	Command       CommandRunner
}

// CommandRunner executes Chutney commands. Unit tests replace it so command
// construction can be checked without starting Tor.
type CommandRunner interface {
	Run(ctx context.Context, dir string, env []string, name string, args ...string) error
}

// Lab owns a private Chutney network and its ephemeral onion services.
type Lab struct {
	opts     Options
	dataDir  string
	chutney  string
	socks    string
	control  string
	services []OnionService
}

// Start initializes and bootstraps a Chutney network.
func Start(ctx context.Context, opts Options) (*Lab, error) {
	if opts.Command == nil {
		opts.Command = execCommandRunner{}
	}
	if opts.Network == "" {
		opts.Network = defaultNetwork
	}
	if opts.BootstrapTime == 0 {
		opts.BootstrapTime = 120 * time.Second
	}
	if opts.ChutneyPath == "" {
		opts.ChutneyPath = os.Getenv("CHUTNEY_SOURCE")
	}
	if opts.ChutneyPath == "" {
		return nil, fmt.Errorf("CHUTNEY_SOURCE is not set")
	}
	if opts.DataDir == "" {
		dir, err := os.MkdirTemp("", "bip157-torlab-*")
		if err != nil {
			return nil, fmt.Errorf("create tor lab dir: %w", err)
		}
		opts.DataDir = dir
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tor lab dir: %w", err)
	}

	lab := &Lab{
		opts:    opts,
		dataDir: opts.DataDir,
		chutney: filepath.Join(opts.ChutneyPath, "chutney"),
	}
	env := []string{
		"CHUTNEY_START_TIME=" + strconv.Itoa(int(opts.BootstrapTime.Seconds())),
		"CHUTNEY_TOR_SANDBOX=0",
	}
	if err := lab.run(ctx, env, "init", "--net", opts.Network); err != nil {
		return nil, err
	}
	if err := lab.run(ctx, env, "bootstrap"); err != nil {
		_ = lab.Close()
		return nil, err
	}
	if err := lab.discoverEndpoints(); err != nil {
		_ = lab.Close()
		return nil, err
	}
	return lab, nil
}

// SOCKSAddress returns the Chutney client's SOCKS endpoint.
func (l *Lab) SOCKSAddress() string {
	return l.socks
}

// ControlAddress returns the Chutney client's TCP control endpoint.
func (l *Lab) ControlAddress() string {
	return l.control
}

// Expose creates one v3 onion service that forwards to localListenAddress.
func (l *Lab) Expose(ctx context.Context, peerIndex int, localListenAddress string) (OnionService, error) {
	host, port, err := net.SplitHostPort(localListenAddress)
	if err != nil {
		return OnionService{}, fmt.Errorf("split peer listen address: %w", err)
	}
	if host == "" || host == "::" || host == "::1" {
		host = "127.0.0.1"
	}
	target := net.JoinHostPort(host, port)
	client := ControlClient{Address: l.control, Timeout: 10 * time.Second}
	service, err := client.AddOnion(ctx, 8333, target)
	if err != nil {
		return OnionService{}, fmt.Errorf("create onion service for peer %d: %w", peerIndex, err)
	}
	l.services = append(l.services, service)
	return service, nil
}

// Close removes ephemeral onion services and stops the Chutney network.
func (l *Lab) Close() error {
	var firstErr error
	if l.control != "" {
		client := ControlClient{Address: l.control, Timeout: 10 * time.Second}
		for i := len(l.services) - 1; i >= 0; i-- {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := client.DelOnion(ctx, l.services[i].ServiceID)
			cancel()
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := l.run(ctx, nil, "stop"); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (l *Lab) run(ctx context.Context, env []string, args ...string) error {
	fullArgs := append([]string{"--data-dir", l.dataDir}, args...)
	if err := l.opts.Command.Run(ctx, l.opts.ChutneyPath, env, l.chutney, fullArgs...); err != nil {
		return fmt.Errorf("chutney %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (l *Lab) discoverEndpoints() error {
	data, err := os.ReadFile(filepath.Join(l.dataDir, "nodes", "network.json"))
	if err != nil {
		return fmt.Errorf("read Chutney network.json: %w", err)
	}
	var network chutneyNetwork
	if err := json.Unmarshal(data, &network); err != nil {
		return fmt.Errorf("decode Chutney network.json: %w", err)
	}
	for _, node := range network.Nodes {
		if !node.IsClient {
			continue
		}
		l.socks = firstIPv4Endpoint(node.SOCKSPortEndpoints)
		l.control = firstIPv4Endpoint(node.ControlPortEndpoints)
		if l.socks == "" || l.control == "" {
			return fmt.Errorf("client node %s missing SOCKS or control endpoint", node.Nick)
		}
		return nil
	}
	return fmt.Errorf("Chutney network has no client node")
}

type chutneyNetwork struct {
	Nodes []chutneyNode `json:"nodes"`
}

type chutneyNode struct {
	Nick                 string   `json:"nick"`
	IsClient             intBool  `json:"is_client"`
	SOCKSPortEndpoints   []string `json:"socksport_endpoints"`
	ControlPortEndpoints []string `json:"controlport_endpoints"`
}

type intBool bool

func (b *intBool) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", "1":
		*b = true
	case "false", "0", "null":
		*b = false
	default:
		return fmt.Errorf("invalid Chutney boolean %s", data)
	}
	return nil
}

func firstIPv4Endpoint(endpoints []string) string {
	for _, endpoint := range endpoints {
		host, _, err := net.SplitHostPort(endpoint)
		if err == nil && net.ParseIP(host).To4() != nil {
			return endpoint
		}
	}
	return ""
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return commandError{err: err, output: string(out)}
	}
	return nil
}

type commandError struct {
	err    error
	output string
}

func (e commandError) Error() string {
	out := strings.TrimSpace(e.output)
	if out == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + out
}

func (e commandError) Unwrap() error {
	return e.err
}
