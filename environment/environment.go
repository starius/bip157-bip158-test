// Package environment names the network environments used by the harness.
//
// BIP157/BIP158 behavior should be the same over IPv4, IPv6, and supported
// overlay transports. Keeping environment metadata in one package lets the
// harness, adapter API, and matrix code agree on names without parsing peer
// address strings.
package environment

import "fmt"

// ID is the stable machine-readable name for one test environment.
type ID string

const (
	// IPv4 is clear TCP over distinct IPv4 loopback addresses.
	IPv4 ID = "ipv4"
	// IPv6 is clear TCP over IPv6 loopback addressing.
	IPv6 ID = "ipv6"
	// TorV3 is TCP carried through Tor v3 onion services.
	TorV3 ID = "tor-v3"
	// I2P is TCP-style streaming carried through I2P destinations.
	I2P ID = "i2p"
	// CJDNS is clear Bitcoin P2P traffic over a cjdns IPv6 overlay.
	CJDNS ID = "cjdns"
)

// Transport describes how the adapter reaches a peer.
type Transport string

const (
	// TransportTCP is a direct TCP socket.
	TransportTCP Transport = "tcp"
	// TransportTorV3 is a Tor v3 onion service reached through SOCKS or
	// implementation-native onion support.
	TransportTorV3 Transport = "tor-v3"
	// TransportI2P is an I2P streaming destination reached through SAM or
	// implementation-native I2P support.
	TransportI2P Transport = "i2p"
	// TransportCJDNS is a cjdns IPv6 overlay address.
	TransportCJDNS Transport = "cjdns"
)

// AddressType describes the peer address namespace.
type AddressType string

const (
	// AddressIPv4 is an IPv4 host and TCP port.
	AddressIPv4 AddressType = "ipv4"
	// AddressIPv6 is an IPv6 host and TCP port.
	AddressIPv6 AddressType = "ipv6"
	// AddressOnionV3 is a Tor v3 onion service address.
	AddressOnionV3 AddressType = "tor-v3"
	// AddressI2P is an I2P destination.
	AddressI2P AddressType = "i2p"
	// AddressCJDNS is a cjdns IPv6 overlay address.
	AddressCJDNS AddressType = "cjdns"
)

// Definition is the harness-wide description of one environment.
type Definition struct {
	ID                     ID
	AddressType            AddressType
	Transport              Transport
	RequiresProxy          bool
	Overlay                bool
	DistinctPeerIdentities bool
	Description            string
}

// DefaultID returns the environment used when no CLI flag is provided.
func DefaultID() ID {
	return IPv4
}

// All returns every environment tracked by Workstream 6.
func All() []Definition {
	return []Definition{
		{
			ID:                     IPv4,
			AddressType:            AddressIPv4,
			Transport:              TransportTCP,
			DistinctPeerIdentities: true,
			Description:            "clear TCP over distinct IPv4 loopback addresses",
		},
		{
			ID:          IPv6,
			AddressType: AddressIPv6,
			Transport:   TransportTCP,
			Description: "clear TCP over IPv6 loopback addressing",
		},
		{
			ID:            TorV3,
			AddressType:   AddressOnionV3,
			Transport:     TransportTorV3,
			RequiresProxy: true,
			Overlay:       true,
			Description:   "Tor v3 onion service through a private Chutney lab",
		},
		{
			ID:            I2P,
			AddressType:   AddressI2P,
			Transport:     TransportI2P,
			RequiresProxy: true,
			Overlay:       true,
			Description:   "I2P streaming destination through a private I2P lab",
		},
		{
			ID:                     CJDNS,
			AddressType:            AddressCJDNS,
			Transport:              TransportCJDNS,
			Overlay:                true,
			DistinctPeerIdentities: true,
			Description:            "cjdns IPv6 overlay address",
		},
	}
}

// Lookup resolves id to an environment definition.
func Lookup(id string) (Definition, error) {
	if id == "" {
		id = string(DefaultID())
	}
	for _, def := range All() {
		if string(def.ID) == id {
			return def, nil
		}
	}
	return Definition{}, fmt.Errorf("unknown environment %q", id)
}

// IDs returns every known environment name.
func IDs() []string {
	defs := All()
	ids := make([]string, 0, len(defs))
	for _, def := range defs {
		ids = append(ids, string(def.ID))
	}
	return ids
}

// IsClearTCP reports whether peerlab can bind this environment directly.
func (d Definition) IsClearTCP() bool {
	return d.Transport == TransportTCP
}
