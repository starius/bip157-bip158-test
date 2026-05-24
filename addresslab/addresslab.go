// Package addresslab allocates local peer addresses for harness scenarios.
//
// Most BIP157/BIP158 behavior can be tested with plain loopback addresses,
// but bad-peer and effective-ban scenarios need peers that are distinguishable
// by address, not only by TCP port. This package keeps that policy outside of
// peerlab so tests can choose between non-privileged and privileged modes.
package addresslab

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/bip157-bip158-test/suite/environment"
)

// Mode names the address allocation backend used by one harness run.
type Mode string

const (
	// ModeAuto chooses the best allocator available to the current process.
	ModeAuto Mode = "auto"
	// ModeLoopback uses addresses that work without elevated privileges.
	ModeLoopback Mode = "loopback"
	// ModeLinuxIPRoute uses the Linux ip command to add temporary IPv6
	// addresses to a local interface.
	ModeLinuxIPRoute Mode = "linux-iproute"
)

// Capabilities describes the identity guarantees provided by an allocator.
type Capabilities struct {
	Mode                   Mode
	DistinctIPv4Identities bool
	DistinctIPv6Identities bool
}

// Lease is one address assigned to a peerlab server.
type Lease struct {
	ID       string
	Host     string
	Distinct bool
	Release  func() error
}

// Allocator assigns one local listen host per peer.
type Allocator interface {
	Allocate(env environment.Definition, peerIndex int) (Lease, error)
	Capabilities() Capabilities
	Close() error
}

// New returns the allocator requested by mode.
func New(mode string) (Allocator, error) {
	switch Mode(mode) {
	case "":
		return NewLoopback(), nil
	case ModeAuto:
		if canUseLinuxIPRoute() {
			return NewLinuxIPRoute(LinuxIPRouteOptions{}), nil
		}
		return NewLoopback(), nil
	case ModeLoopback:
		return NewLoopback(), nil
	case ModeLinuxIPRoute:
		return NewLinuxIPRoute(LinuxIPRouteOptions{}), nil
	default:
		return nil, fmt.Errorf("unknown address lab %q", mode)
	}
}

// NewLoopback returns a non-privileged allocator.
func NewLoopback() Allocator {
	return loopbackAllocator{}
}

type loopbackAllocator struct{}

func (loopbackAllocator) Allocate(env environment.Definition, peerIndex int) (Lease, error) {
	if peerIndex < 1 {
		return Lease{}, fmt.Errorf("peer index must be positive")
	}
	switch env.ID {
	case environment.IPv4:
		return Lease{
			ID:       fmt.Sprintf("%s-peer-%d", env.ID, peerIndex),
			Host:     fmt.Sprintf("127.27.0.%d", peerIndex),
			Distinct: true,
		}, nil
	case environment.IPv6:
		return Lease{
			ID:       fmt.Sprintf("%s-peer-%d", env.ID, peerIndex),
			Host:     "::1",
			Distinct: false,
		}, nil
	default:
		return Lease{}, fmt.Errorf("%s requires an overlay lab", env.ID)
	}
}

func (loopbackAllocator) Capabilities() Capabilities {
	return Capabilities{
		Mode:                   ModeLoopback,
		DistinctIPv4Identities: true,
		DistinctIPv6Identities: false,
	}
}

func (loopbackAllocator) Close() error { return nil }

// LinuxIPRouteOptions customizes the privileged Linux allocator.
type LinuxIPRouteOptions struct {
	Command  CommandRunner
	IPBinary string
	Device   string
	Prefix   string
}

// CommandRunner executes iproute2 commands. Tests use it to validate command
// planning without changing the host network configuration.
type CommandRunner interface {
	Run(name string, args ...string) error
}

// NewLinuxIPRoute returns an allocator that adds one IPv6 /128 per peer.
func NewLinuxIPRoute(opts LinuxIPRouteOptions) Allocator {
	if opts.Command == nil {
		opts.Command = execRunner{}
	}
	if opts.IPBinary == "" {
		opts.IPBinary = "ip"
	}
	if opts.Device == "" {
		opts.Device = "lo"
	}
	if opts.Prefix == "" {
		opts.Prefix = "fd7a:b157:b158"
	}
	return &linuxIPRouteAllocator{opts: opts}
}

type linuxIPRouteAllocator struct {
	opts   LinuxIPRouteOptions
	leases []Lease
}

func (a *linuxIPRouteAllocator) Allocate(env environment.Definition, peerIndex int) (Lease, error) {
	if peerIndex < 1 {
		return Lease{}, fmt.Errorf("peer index must be positive")
	}
	if env.ID == environment.IPv4 {
		return loopbackAllocator{}.Allocate(env, peerIndex)
	}
	if env.ID != environment.IPv6 {
		return Lease{}, fmt.Errorf("%s requires an overlay lab", env.ID)
	}

	host := fmt.Sprintf("%s::%x", a.opts.Prefix, peerIndex)
	if ip := net.ParseIP(host); ip == nil || ip.To4() != nil {
		return Lease{}, fmt.Errorf("invalid generated IPv6 address %q", host)
	}
	cidr := host + "/128"
	if err := a.run("addr", "add", cidr, "dev", a.opts.Device); err != nil {
		if !isAlreadyExists(err) {
			return Lease{}, fmt.Errorf("add %s to %s: %w", cidr, a.opts.Device, err)
		}
	}
	lease := Lease{
		ID:       fmt.Sprintf("%s-peer-%d", env.ID, peerIndex),
		Host:     host,
		Distinct: true,
	}
	lease.Release = func() error {
		if err := a.run("addr", "del", cidr, "dev", a.opts.Device); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete %s from %s: %w", cidr, a.opts.Device, err)
		}
		return nil
	}
	a.leases = append(a.leases, lease)
	return lease, nil
}

func (a *linuxIPRouteAllocator) Capabilities() Capabilities {
	return Capabilities{
		Mode:                   ModeLinuxIPRoute,
		DistinctIPv4Identities: true,
		DistinctIPv6Identities: true,
	}
}

func (a *linuxIPRouteAllocator) Close() error {
	var joined error
	for i := len(a.leases) - 1; i >= 0; i-- {
		if a.leases[i].Release != nil {
			joined = errors.Join(joined, a.leases[i].Release())
		}
	}
	a.leases = nil
	return joined
}

func (a *linuxIPRouteAllocator) run(args ...string) error {
	return a.opts.Command.Run(a.opts.IPBinary, args...)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
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

func canUseLinuxIPRoute() bool {
	if os.Geteuid() != 0 {
		return false
	}
	_, err := exec.LookPath("ip")
	return err == nil
}

func isAlreadyExists(err error) bool {
	return strings.Contains(err.Error(), "File exists")
}

func isNotFound(err error) bool {
	text := err.Error()
	return strings.Contains(text, "Cannot assign requested address") ||
		strings.Contains(text, "No such process") ||
		strings.Contains(text, "Cannot find")
}
