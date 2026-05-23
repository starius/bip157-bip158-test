// Package chainlab builds deterministic block/filter fixtures for the harness.
package chainlab

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// FilterMaterial is the data a BIP157 peer serves for one block.
type FilterMaterial struct {
	BlockHash    chainhash.Hash
	Filter       *gcs.Filter
	FilterBytes  []byte
	FilterHash   chainhash.Hash
	FilterHeader chainhash.Hash
}

// BuildBasicFilter constructs a BIP158 basic filter from an explicit block and
// prevout-script set. The function is intentionally small and auditable because
// the suite must not rely on an implementation under test for expected data.
func BuildBasicFilter(block *wire.MsgBlock, prevOutScripts [][]byte) (*gcs.Filter, []byte, error) {
	filter, err := builder.BuildBasicFilter(block, prevOutScripts)
	if err != nil {
		return nil, nil, fmt.Errorf("build basic filter: %w", err)
	}
	serialized, err := filter.NBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("serialize filter: %w", err)
	}
	return filter, serialized, nil
}

// BuildFilterMaterial derives the filter hash and filter header for a block.
func BuildFilterMaterial(block *wire.MsgBlock, prevOutScripts [][]byte, prevFilterHeader chainhash.Hash) (*FilterMaterial, error) {
	filter, serialized, err := BuildBasicFilter(block, prevOutScripts)
	if err != nil {
		return nil, err
	}
	filterHash, err := builder.GetFilterHash(filter)
	if err != nil {
		return nil, fmt.Errorf("hash filter: %w", err)
	}
	filterHeader, err := builder.MakeHeaderForFilter(filter, prevFilterHeader)
	if err != nil {
		return nil, fmt.Errorf("build filter header: %w", err)
	}
	return &FilterMaterial{
		BlockHash:    block.BlockHash(),
		Filter:       filter,
		FilterBytes:  serialized,
		FilterHash:   filterHash,
		FilterHeader: filterHeader,
	}, nil
}

// Contains reports whether a serialized filter matches script for blockHash.
func Contains(serialized []byte, blockHash chainhash.Hash, script []byte) (bool, error) {
	filter, err := gcs.FromNBytes(builder.DefaultP, builder.DefaultM, serialized)
	if err != nil {
		return false, fmt.Errorf("parse filter: %w", err)
	}
	key := builder.DeriveKey(&blockHash)
	return filter.Match(key, script)
}

// Hex encodes bytes for stable JSON reports and test names.
func Hex(b []byte) string {
	return hex.EncodeToString(b)
}

// EqualBytes is a named helper to keep tests readable for non-Go readers.
func EqualBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}
