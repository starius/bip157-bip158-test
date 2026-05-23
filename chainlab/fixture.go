package chainlab

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// MatchExpectation is the harness-side truth for one wallet-relevant
// transaction. Adapters are judged against these values instead of against
// another light-client implementation.
type MatchExpectation struct {
	ScriptPubKey []byte
	TxID         chainhash.Hash
	BlockHash    chainhash.Hash
	Height       uint32
	Kind         string
	Vout         uint32
	Vin          uint32
}

// BlockFixture groups a block with the BIP158 material peers will serve.
type BlockFixture struct {
	Height         uint32
	Block          *wire.MsgBlock
	PrevOutScripts [][]byte
	Filter         *FilterMaterial
}

// Fixture is a short deterministic regtest chain with wallet activity.
type Fixture struct {
	Params        *chaincfg.Params
	Blocks        []BlockFixture
	WatchedScript []byte
	Matches       []MatchExpectation
}

// BuildWalletFixture creates a regtest chain that exercises the minimum wallet
// behavior every BIP157/BIP158 client should get right: detect a watched output
// and then detect a spend of that output through the prevout-script element.
func BuildWalletFixture() (*Fixture, error) {
	params := &chaincfg.RegressionNetParams
	// A deterministic P2WPKH script keeps the fixture standard enough for
	// wallet-style APIs while still avoiding any private key dependency.
	watched := append([]byte{0x00, 0x14}, []byte{
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
	}...)
	anyone := []byte{0x51}

	genesis := cloneBlock(params.GenesisBlock)
	genesisMaterial, err := BuildFilterMaterial(genesis, nil, chainhash.Hash{})
	if err != nil {
		return nil, err
	}

	var blocks []BlockFixture
	blocks = append(blocks, BlockFixture{
		Height: 0,
		Block:  genesis,
		Filter: genesisMaterial,
	})

	prevHeader := genesisMaterial.FilterHeader
	prevHash := genesis.BlockHash()
	prevCoinbase := genesis.Transactions[0].TxHash()

	receiveTx := &wire.MsgTx{
		Version: 2,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevCoinbase, Index: 0},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []*wire.TxOut{{
			Value:    1_000,
			PkScript: watched,
		}},
	}
	block1, err := mineBlock(params, 1, prevHash, []*wire.MsgTx{receiveTx})
	if err != nil {
		return nil, err
	}
	material1, err := BuildFilterMaterial(block1, [][]byte{genesis.Transactions[0].TxOut[0].PkScript}, prevHeader)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, BlockFixture{
		Height:         1,
		Block:          block1,
		PrevOutScripts: [][]byte{genesis.Transactions[0].TxOut[0].PkScript},
		Filter:         material1,
	})

	prevHeader = material1.FilterHeader
	prevHash = block1.BlockHash()
	receiveOut := wire.OutPoint{Hash: receiveTx.TxHash(), Index: 0}
	spendTx := &wire.MsgTx{
		Version: 2,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: receiveOut,
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []*wire.TxOut{{
			Value:    900,
			PkScript: anyone,
		}},
	}
	block2, err := mineBlock(params, 2, prevHash, []*wire.MsgTx{spendTx})
	if err != nil {
		return nil, err
	}
	material2, err := BuildFilterMaterial(block2, [][]byte{watched}, prevHeader)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, BlockFixture{
		Height:         2,
		Block:          block2,
		PrevOutScripts: [][]byte{watched},
		Filter:         material2,
	})

	return &Fixture{
		Params:        params,
		Blocks:        blocks,
		WatchedScript: watched,
		Matches: []MatchExpectation{{
			ScriptPubKey: watched,
			TxID:         receiveTx.TxHash(),
			BlockHash:    block1.BlockHash(),
			Height:       1,
			Kind:         "output",
			Vout:         0,
		}, {
			ScriptPubKey: watched,
			TxID:         spendTx.TxHash(),
			BlockHash:    block2.BlockHash(),
			Height:       2,
			Kind:         "spend",
			Vin:          0,
		}},
	}, nil
}

func cloneBlock(block *wire.MsgBlock) *wire.MsgBlock {
	copy := block.Copy()
	return copy
}

func mineBlock(params *chaincfg.Params, height int32, prevHash chainhash.Hash, txs []*wire.MsgTx) (*wire.MsgBlock, error) {
	coinbase := &wire.MsgTx{
		Version: 2,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{byte(height), 0x51},
			Sequence:        0xffffffff,
		}},
		TxOut: []*wire.TxOut{{
			Value:    50_0000_0000,
			PkScript: []byte{0x51},
		}},
	}

	allTx := append([]*wire.MsgTx{coinbase}, txs...)
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prevHash,
			Bits:      params.PowLimitBits,
			Timestamp: time.Unix(1_700_000_000+int64(height)*600, 0),
		},
		Transactions: allTx,
	}

	utilTxs := make([]*btcutil.Tx, 0, len(allTx))
	for _, tx := range allTx {
		utilTxs = append(utilTxs, btcutil.NewTx(tx))
	}
	block.Header.MerkleRoot = blockchain.CalcMerkleRoot(utilTxs, false)

	target := blockchain.CompactToBig(params.PowLimitBits)
	for nonce := uint32(0); nonce < ^uint32(0); nonce++ {
		block.Header.Nonce = nonce
		hash := block.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			return block, nil
		}
	}
	return nil, fmt.Errorf("unable to mine regtest block at height %d", height)
}
