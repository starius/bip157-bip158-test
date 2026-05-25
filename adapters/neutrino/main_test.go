package main

import (
	"encoding/hex"
	"testing"

	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestP2WPKHAddressAcceptsFixtureScript(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	addr, err := p2wpkhAddress(fixture.WatchedScript)
	if err != nil {
		t.Fatalf("parse fixture script: %v", err)
	}
	if addr.String() == "" {
		t.Fatalf("address string is empty")
	}
}

func TestP2WPKHAddressRejectsUnsupportedScripts(t *testing.T) {
	if _, err := p2wpkhAddress([]byte{0x51}); err == nil {
		t.Fatalf("expected unsupported script error")
	}
}

func TestCapabilitiesSupportIPv4AndIPv6(t *testing.T) {
	caps := adapterCapabilities()
	if len(caps.Environments) != 5 {
		t.Fatalf("capabilities = %d, want 5", len(caps.Environments))
	}
	for _, cap := range caps.Environments {
		switch cap.ID {
		case "ipv4", "ipv6":
			if !cap.Supported {
				t.Fatalf("%s should be supported", cap.ID)
			}
		default:
			if cap.Supported {
				t.Fatalf("%s should be unsupported until validated", cap.ID)
			}
		}
	}
}

func TestPeerStatePreservesIPv6Metadata(t *testing.T) {
	state := peerStateFromConfig(api.PeerConfig{
		ID:          "ipv6-peer",
		Address:     "[fd7a:b157:b158::1]:18444",
		AddressType: "ipv6",
		Transport:   "tcp",
		Identity:    "fd7a:b157:b158::1",
		Trusted:     true,
	}, true)

	if state.Address != "[fd7a:b157:b158::1]:18444" {
		t.Fatalf("address = %s", state.Address)
	}
	if state.AddressType != "ipv6" || state.Transport != "tcp" {
		t.Fatalf("metadata = %s/%s", state.AddressType, state.Transport)
	}
	if state.Identity != "fd7a:b157:b158::1" {
		t.Fatalf("identity = %s", state.Identity)
	}
	if !state.Connected {
		t.Fatalf("connected flag was not preserved")
	}
}

func TestRecordFilteredBlockFindsOutputsAndSpends(t *testing.T) {
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	a := newAdapter()
	scriptHex := hex.EncodeToString(fixture.WatchedScript)
	a.outpoints[scriptHex] = map[wire.OutPoint]struct{}{}

	for _, block := range fixture.Blocks[1:] {
		var txs []*btcutil.Tx
		for _, tx := range block.Block.Transactions {
			txs = append(txs, btcutil.NewTx(tx))
		}
		a.recordFilteredBlock(scriptHex, block.Height, block.Block.BlockHash(), txs)
	}

	if len(a.matches) != 2 {
		t.Fatalf("got %d matches, want 2: %+v", len(a.matches), a.matches)
	}
	if a.matches[0].Kind != api.MatchKindOutput {
		t.Fatalf("first match kind = %s, want output", a.matches[0].Kind)
	}
	if a.matches[1].Kind != api.MatchKindSpend {
		t.Fatalf("second match kind = %s, want spend", a.matches[1].Kind)
	}
}

func TestAddMatchDeduplicates(t *testing.T) {
	a := newAdapter()
	match := api.TxMatch{
		TxIDHex:      chainhash.Hash{}.String(),
		BlockHashHex: chainhash.Hash{}.String(),
		Kind:         api.MatchKindOutput,
	}
	a.addMatchLocked(match)
	a.addMatchLocked(match)
	if len(a.matches) != 1 {
		t.Fatalf("duplicate match was recorded")
	}
}
