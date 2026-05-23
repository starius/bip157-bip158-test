// Package api contains the stable HTTP/JSON representation of the adapter API.
//
// The project also ships proto/bip157test.proto as the canonical schema. The
// first implementation uses an HTTP JSON mapping so adapters can be small and
// easy to write in any language; field names mirror the proto messages.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PeerConfig identifies one harness-controlled Bitcoin P2P peer.
type PeerConfig struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Trusted bool   `json:"trusted"`
}

// ConfigureRequest initializes an adapter for one isolated conformance run.
type ConfigureRequest struct {
	Network        string       `json:"network"`
	DataDir        string       `json:"data_dir"`
	Peers          []PeerConfig `json:"peers"`
	RequiredPeers  uint32       `json:"required_peers"`
	AllowDiscovery bool         `json:"allow_discovery"`
}

// BlockRef is a hash/height pair on the adapter's best known chain.
type BlockRef struct {
	HashHex string `json:"hash_hex"`
	Height  uint32 `json:"height"`
}

// WatchScriptRequest asks the adapter to track a scriptPubKey from a height.
type WatchScriptRequest struct {
	ScriptPubKeyHex string `json:"script_pubkey_hex"`
	StartHeight     uint32 `json:"start_height"`
}

// MatchKind classifies why a transaction is relevant to a watched script.
type MatchKind string

const (
	// MatchKindOutput means the transaction created a watched output.
	MatchKindOutput MatchKind = "output"
	// MatchKindSpend means the transaction spent a previously watched output.
	MatchKindSpend MatchKind = "spend"
)

// TxMatch is the normalized wallet-relevance result reported by an adapter.
type TxMatch struct {
	TxIDHex      string    `json:"txid_hex"`
	BlockHashHex string    `json:"block_hash_hex"`
	Height       uint32    `json:"height"`
	Kind         MatchKind `json:"kind"`
	Vout         uint32    `json:"vout,omitempty"`
	Vin          uint32    `json:"vin,omitempty"`
}

// GetMatchesRequest queries all known matches for a watched script.
type GetMatchesRequest struct {
	ScriptPubKeyHex string `json:"script_pubkey_hex"`
	StartHeight     uint32 `json:"start_height"`
	StopHeight      uint32 `json:"stop_height"`
}

// GetMatchesResponse is returned by the adapter's match query endpoint.
type GetMatchesResponse struct {
	Matches []TxMatch `json:"matches"`
}

// PeerState exposes the adapter's current view of one peer.
type PeerState struct {
	ID          string `json:"id"`
	Address     string `json:"address"`
	Connected   bool   `json:"connected"`
	Banned      bool   `json:"banned"`
	LastError   string `json:"last_error,omitempty"`
	BestHeight  uint32 `json:"best_height,omitempty"`
	BestHashHex string `json:"best_hash_hex,omitempty"`
}

// ListPeersResponse contains every peer the adapter is willing to report.
type ListPeersResponse struct {
	Peers []PeerState `json:"peers"`
}

// HealthResponse is intentionally boring: the harness only needs to know
// whether the adapter process is alive and what state it thinks it is in.
type HealthResponse struct {
	Alive  bool   `json:"alive"`
	Status string `json:"status"`
}

// Client is a tiny typed HTTP client used by the harness to call adapters.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns an adapter client rooted at baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PostJSON sends a JSON request and decodes a JSON response.
func (c *Client) PostJSON(ctx context.Context, path string, req, resp any) error {
	var body bytes.Buffer
	if req != nil {
		if err := json.NewEncoder(&body).Encode(req); err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("post %s: status %s", path, httpResp.Status)
	}
	if resp == nil {
		return nil
	}
	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
