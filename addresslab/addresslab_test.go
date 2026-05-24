package addresslab

import (
	"errors"
	"reflect"
	"testing"

	"github.com/bip157-bip158-test/suite/environment"
)

func TestLoopbackAllocatorUsesDistinctIPv4AndSharedIPv6(t *testing.T) {
	alloc := NewLoopback()

	ipv4, err := environment.Lookup("ipv4")
	if err != nil {
		t.Fatalf("lookup ipv4: %v", err)
	}
	first, err := alloc.Allocate(ipv4, 1)
	if err != nil {
		t.Fatalf("allocate first ipv4: %v", err)
	}
	second, err := alloc.Allocate(ipv4, 2)
	if err != nil {
		t.Fatalf("allocate second ipv4: %v", err)
	}
	if first.Host == second.Host {
		t.Fatalf("ipv4 loopback hosts should be distinct, both are %s", first.Host)
	}
	if !first.Distinct || !second.Distinct {
		t.Fatalf("ipv4 loopback leases should be marked distinct")
	}

	ipv6, err := environment.Lookup("ipv6")
	if err != nil {
		t.Fatalf("lookup ipv6: %v", err)
	}
	lease, err := alloc.Allocate(ipv6, 1)
	if err != nil {
		t.Fatalf("allocate ipv6: %v", err)
	}
	if lease.Host != "::1" || lease.Distinct {
		t.Fatalf("ipv6 fallback lease = %+v, want shared ::1", lease)
	}
}

func TestLinuxIPRouteAllocatorAddsAndDeletesIPv6(t *testing.T) {
	runner := &recordingRunner{}
	alloc := NewLinuxIPRoute(LinuxIPRouteOptions{
		Command:  runner,
		IPBinary: "ip",
		Device:   "lo-test",
		Prefix:   "fd7a:b157:b158",
	})
	ipv6, err := environment.Lookup("ipv6")
	if err != nil {
		t.Fatalf("lookup ipv6: %v", err)
	}

	lease, err := alloc.Allocate(ipv6, 10)
	if err != nil {
		t.Fatalf("allocate ipv6: %v", err)
	}
	if lease.Host != "fd7a:b157:b158::a" || !lease.Distinct {
		t.Fatalf("lease = %+v", lease)
	}
	if err := alloc.Close(); err != nil {
		t.Fatalf("close allocator: %v", err)
	}

	want := [][]string{
		{"ip", "addr", "add", "fd7a:b157:b158::a/128", "dev", "lo-test"},
		{"ip", "addr", "del", "fd7a:b157:b158::a/128", "dev", "lo-test"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestLinuxIPRouteAllocatorRejectsFailedAdd(t *testing.T) {
	runner := &recordingRunner{err: errors.New("permission denied")}
	alloc := NewLinuxIPRoute(LinuxIPRouteOptions{Command: runner})
	ipv6, err := environment.Lookup("ipv6")
	if err != nil {
		t.Fatalf("lookup ipv6: %v", err)
	}

	if _, err := alloc.Allocate(ipv6, 1); err == nil {
		t.Fatalf("failed add unexpectedly succeeded")
	}
}

func TestNewRejectsUnknownMode(t *testing.T) {
	if _, err := New("bogus"); err == nil {
		t.Fatalf("unknown mode unexpectedly succeeded")
	}
}

type recordingRunner struct {
	calls [][]string
	err   error
}

func (r *recordingRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	return r.err
}
