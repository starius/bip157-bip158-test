package chainlab

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildBasicFilterIncludesCoinbaseOutputs(t *testing.T) {
	watched := []byte{0x51} // OP_TRUE is enough for filter membership tests.
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{},
		Transactions: []*wire.MsgTx{{
			Version: 1,
			TxIn: []*wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{0x01, 0x01},
				Sequence:        0xffffffff,
			}},
			TxOut: []*wire.TxOut{{
				Value:    50_0000_0000,
				PkScript: watched,
			}},
		}},
	}

	_, serialized, err := BuildBasicFilter(block, nil)
	if err != nil {
		t.Fatalf("build filter: %v", err)
	}

	matches, err := Contains(serialized, block.BlockHash(), watched)
	if err != nil {
		t.Fatalf("match filter: %v", err)
	}
	if !matches {
		t.Fatalf("coinbase output script was not included in the basic filter")
	}
}

func TestBuildBasicFilterExcludesOPReturn(t *testing.T) {
	opReturn := []byte{0x6a, 0x01, 0x01}
	serialized, _, err := BuildSingleOutputFilter(opReturn)
	if err != nil {
		t.Fatalf("build filter: %v", err)
	}
	if len(serialized) != 1 || serialized[0] != 0 {
		t.Fatalf("expected zero-element filter serialization, got %x", serialized)
	}
}

func TestBuildFilterMaterialLinksHeaders(t *testing.T) {
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{},
		Transactions: []*wire.MsgTx{{
			Version: 1,
			TxIn: []*wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{0x01, 0x01},
				Sequence:        0xffffffff,
			}},
			TxOut: []*wire.TxOut{{
				Value:    1,
				PkScript: []byte{0x51},
			}},
		}},
	}

	material, err := BuildFilterMaterial(block, nil, chainhash.Hash{})
	if err != nil {
		t.Fatalf("build material: %v", err)
	}
	if material.FilterHeader == (chainhash.Hash{}) {
		t.Fatalf("filter header must not be the zero hash for a non-empty filter")
	}
}
