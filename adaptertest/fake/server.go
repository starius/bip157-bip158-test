// Package fake provides a tiny conformant adapter used to test the harness.
//
// It does not implement a light client. Instead, it exposes the adapter API
// over a deterministic chainlab fixture so the harness can be tested end to end
// without depending on Kyoto, Neutrino, or any network timing.
package fake

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
)

// Server handles the adapter HTTP API for a fixed wallet fixture.
type Server struct {
	fixture *chainlab.Fixture

	mu         sync.Mutex
	configured bool
	started    bool
	peers      []api.PeerConfig
	watches    map[string]api.WatchScriptRequest
}

// NewServer returns a fake adapter bound to fixture.
func NewServer(fixture *chainlab.Fixture) *Server {
	return &Server{
		fixture: fixture,
		watches: map[string]api.WatchScriptRequest{},
	}
}

// Handler returns the HTTP routes expected by the conformance harness.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/configure", s.handleConfigure)
	mux.HandleFunc("/start", s.handleStart)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/watch-script", s.handleWatchScript)
	mux.HandleFunc("/best-block", s.handleBestBlock)
	mux.HandleFunc("/block-hash", s.handleBlockHash)
	mux.HandleFunc("/matches", s.handleMatches)
	mux.HandleFunc("/list-peers", s.handleListPeers)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	s.mu.Lock()
	started := s.started
	configured := s.configured
	s.mu.Unlock()

	status := "idle"
	if configured {
		status = "configured"
	}
	if started {
		status = "started"
	}
	writeJSON(w, api.HealthResponse{Alive: true, Status: status})
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.ConfigureRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Network != "regtest" {
		http.Error(w, "fake adapter only supports regtest", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.configured = true
	s.peers = append([]api.PeerConfig(nil), req.Peers...)
	s.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.configured {
		http.Error(w, "configure before start", http.StatusConflict)
		return
	}
	s.started = true
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleWatchScript(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.WatchScriptRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if _, err := hex.DecodeString(req.ScriptPubKeyHex); err != nil {
		http.Error(w, fmt.Sprintf("invalid script hex: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.watches[req.ScriptPubKeyHex] = req
	s.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleBestBlock(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	tip := s.fixture.Blocks[len(s.fixture.Blocks)-1]
	writeJSON(w, api.BlockRef{
		HashHex: tip.Block.BlockHash().String(),
		Height:  tip.Height,
	})
}

func (s *Server) handleBlockHash(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.BlockRef
	if !decodeJSON(w, r, &req) {
		return
	}
	if int(req.Height) >= len(s.fixture.Blocks) {
		http.Error(w, "height out of range", http.StatusNotFound)
		return
	}
	block := s.fixture.Blocks[req.Height]
	writeJSON(w, api.BlockRef{
		HashHex: block.Block.BlockHash().String(),
		Height:  block.Height,
	})
}

func (s *Server) handleMatches(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.GetMatchesRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	s.mu.Lock()
	watch, watched := s.watches[req.ScriptPubKeyHex]
	s.mu.Unlock()
	if !watched {
		writeJSON(w, api.GetMatchesResponse{})
		return
	}

	var matches []api.TxMatch
	for _, expected := range s.fixture.Matches {
		if hex.EncodeToString(expected.ScriptPubKey) != req.ScriptPubKeyHex {
			continue
		}
		if expected.Height < req.StartHeight || expected.Height < watch.StartHeight || expected.Height > req.StopHeight {
			continue
		}
		matches = append(matches, api.TxMatch{
			TxIDHex:      expected.TxID.String(),
			BlockHashHex: expected.BlockHash.String(),
			Height:       expected.Height,
			Kind:         api.MatchKind(expected.Kind),
			Vout:         expected.Vout,
			Vin:          expected.Vin,
		})
	}
	writeJSON(w, api.GetMatchesResponse{Matches: matches})
}

func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	s.mu.Lock()
	started := s.started
	peers := append([]api.PeerConfig(nil), s.peers...)
	s.mu.Unlock()

	tip := s.fixture.Blocks[len(s.fixture.Blocks)-1]
	states := make([]api.PeerState, 0, len(peers))
	for _, peer := range peers {
		states = append(states, api.PeerState{
			ID:          peer.ID,
			Address:     peer.Address,
			Connected:   started,
			Banned:      false,
			BestHeight:  tip.Height,
			BestHashHex: tip.Block.BlockHash().String(),
		})
	}
	writeJSON(w, api.ListPeersResponse{Peers: states})
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, fmt.Sprintf("decode json: %v", err), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, resp any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
