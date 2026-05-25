// Command neutrino-adapter exposes Lightning Labs Neutrino through the fixed
// BIP157/BIP158 conformance API.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bip157-bip158-test/suite/api"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
)

// adapter is a long-running HTTP server around one Neutrino ChainService.
type adapter struct {
	mu sync.Mutex

	config  api.ConfigureRequest
	service *neutrino.ChainService
	db      walletdb.DB
	dataDir string

	rescanQuit chan struct{}
	watches    map[string]api.WatchScriptRequest
	matches    []api.TxMatch
	seen       map[string]struct{}
	outpoints  map[string]map[wire.OutPoint]struct{}
}

func newAdapter() *adapter {
	return &adapter{
		watches:   map[string]api.WatchScriptRequest{},
		seen:      map[string]struct{}{},
		outpoints: map[string]map[wire.OutPoint]struct{}{},
	}
}

func adapterCapabilities() api.CapabilitiesResponse {
	return api.CapabilitiesResponse{Environments: []api.EnvironmentCapability{
		{ID: "ipv4", Supported: true},
		{ID: "ipv6", Supported: true},
		{ID: "tor-v3", Supported: false, Reason: "adapter does not configure Tor proxying"},
		{ID: "i2p", Supported: false, Reason: "adapter does not configure I2P proxying"},
		{ID: "cjdns", Supported: false, Reason: "adapter has not been validated with cjdns"},
	}}
}

func (a *adapter) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/capabilities", a.handleCapabilities)
	mux.HandleFunc("/configure", a.handleConfigure)
	mux.HandleFunc("/start", a.handleStart)
	mux.HandleFunc("/stop", a.handleStop)
	mux.HandleFunc("/watch-script", a.handleWatchScript)
	mux.HandleFunc("/best-block", a.handleBestBlock)
	mux.HandleFunc("/block-hash", a.handleBlockHash)
	mux.HandleFunc("/matches", a.handleMatches)
	mux.HandleFunc("/list-peers", a.handleListPeers)
	return mux
}

func (a *adapter) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	writeJSON(w, adapterCapabilities())
}

func (a *adapter) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	a.mu.Lock()
	running := a.service != nil
	a.mu.Unlock()
	status := "idle"
	if running {
		status = "configured"
	}
	writeJSON(w, api.HealthResponse{Alive: true, Status: status})
}

func (a *adapter) handleConfigure(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.ConfigureRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Network != "regtest" {
		http.Error(w, "only regtest is supported", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.stopLocked(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dataDir := req.DataDir
	if dataDir == "" {
		dir, err := os.MkdirTemp("", "neutrino-adapter-*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dataDir = dir
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db, err := walletdb.Create("bdb", filepath.Join(dataDir, "filters.db"), true, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	peers := make([]string, 0, len(req.Peers))
	for _, peer := range req.Peers {
		peers = append(peers, peer.Address)
	}

	if req.RequiredPeers > 0 {
		neutrino.MaxPeers = int(req.RequiredPeers)
	}
	neutrino.BanDuration = 5 * time.Second
	neutrino.QueryPeerConnectTimeout = 5 * time.Second

	service, err := neutrino.NewChainService(neutrino.Config{
		DataDir:       dataDir,
		Database:      db,
		ChainParams:   chaincfg.RegressionNetParams,
		ConnectPeers:  peers,
		PersistToDisk: true,
	})
	if err != nil {
		_ = db.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.config = req
	a.service = service
	a.db = db
	a.dataDir = dataDir
	a.rescanQuit = nil
	a.watches = map[string]api.WatchScriptRequest{}
	a.matches = nil
	a.seen = map[string]struct{}{}
	a.outpoints = map[string]map[wire.OutPoint]struct{}{}
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *adapter) handleStart(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	a.mu.Lock()
	service := a.service
	a.mu.Unlock()
	if service == nil {
		http.Error(w, "configure before start", http.StatusConflict)
		return
	}
	if err := service.Start(context.Background()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *adapter) handleStop(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	a.mu.Lock()
	err := a.stopLocked()
	a.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *adapter) handleWatchScript(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.WatchScriptRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	script, err := hex.DecodeString(req.ScriptPubKeyHex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	addr, err := p2wpkhAddress(script)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	service := a.service
	a.watches[req.ScriptPubKeyHex] = req
	if a.outpoints[req.ScriptPubKeyHex] == nil {
		a.outpoints[req.ScriptPubKeyHex] = map[wire.OutPoint]struct{}{}
	}
	a.mu.Unlock()
	if service == nil {
		http.Error(w, "configure before watch", http.StatusConflict)
		return
	}

	quit := make(chan struct{})
	rescan := neutrino.NewRescan(
		&neutrino.RescanChainSource{ChainService: service},
		neutrino.QuitChan(quit),
		neutrino.WatchAddrs(addr),
		neutrino.StartBlock(&headerfs.BlockStamp{Height: int32(req.StartHeight)}),
		neutrino.NotificationHandlers(rpcclient.NotificationHandlers{
			OnFilteredBlockConnected: func(height int32, details *wire.BlockHeader, txs []*btcutil.Tx) {
				a.recordFilteredBlock(req.ScriptPubKeyHex, uint32(height), details.BlockHash(), txs)
			},
		}),
	)

	a.mu.Lock()
	if a.rescanQuit != nil {
		close(a.rescanQuit)
	}
	a.rescanQuit = quit
	a.mu.Unlock()
	go func() {
		<-rescan.Start()
		rescan.WaitForShutdown()
	}()
	writeJSON(w, map[string]bool{"ok": true})
}

func (a *adapter) handleBestBlock(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	a.mu.Lock()
	service := a.service
	a.mu.Unlock()
	if service == nil {
		http.Error(w, "not configured", http.StatusConflict)
		return
	}
	best, err := service.BestBlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, api.BlockRef{HashHex: best.Hash.String(), Height: uint32(best.Height)})
}

func (a *adapter) handleBlockHash(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.BlockRef
	if !decodeJSON(w, r, &req) {
		return
	}
	a.mu.Lock()
	service := a.service
	a.mu.Unlock()
	if service == nil {
		http.Error(w, "not configured", http.StatusConflict)
		return
	}
	hash, err := service.GetBlockHash(int64(req.Height))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, api.BlockRef{HashHex: hash.String(), Height: req.Height})
}

func (a *adapter) handleMatches(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req api.GetMatchesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []api.TxMatch
	for _, match := range a.matches {
		if match.Height < req.StartHeight || match.Height > req.StopHeight {
			continue
		}
		out = append(out, match)
	}
	writeJSON(w, api.GetMatchesResponse{Matches: out})
}

func (a *adapter) handleListPeers(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	a.mu.Lock()
	service := a.service
	configured := append([]api.PeerConfig(nil), a.config.Peers...)
	a.mu.Unlock()
	if service == nil {
		writeJSON(w, api.ListPeersResponse{})
		return
	}

	connected := map[string]struct{}{}
	for _, peer := range service.Peers() {
		connected[peer.Addr()] = struct{}{}
	}
	states := make([]api.PeerState, 0, len(configured))
	for _, peer := range configured {
		_, ok := connected[peer.Address]
		state := peerStateFromConfig(peer, ok)
		if !ok {
			state.LastError = "not connected"
		}
		states = append(states, state)
	}
	writeJSON(w, api.ListPeersResponse{Peers: states})
}

func peerStateFromConfig(peer api.PeerConfig, connected bool) api.PeerState {
	return api.PeerState{
		ID:          peer.ID,
		Address:     peer.Address,
		AddressType: peer.AddressType,
		Transport:   peer.Transport,
		Identity:    peer.Identity,
		Connected:   connected,
		Banned:      false,
	}
}

func (a *adapter) stopLocked() error {
	var errs []error
	if a.rescanQuit != nil {
		close(a.rescanQuit)
		a.rescanQuit = nil
	}
	if a.service != nil {
		errs = append(errs, a.service.Stop())
		a.service = nil
	}
	if a.db != nil {
		errs = append(errs, a.db.Close())
		a.db = nil
	}
	return errors.Join(errs...)
}

func (a *adapter) recordFilteredBlock(scriptHex string, height uint32, blockHash chainhash.Hash, txs []*btcutil.Tx) {
	script, err := hex.DecodeString(scriptHex)
	if err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tx := range txs {
		msgTx := tx.MsgTx()
		txHash := tx.Hash()
		for vout, output := range msgTx.TxOut {
			if !equalBytes(output.PkScript, script) {
				continue
			}
			a.outpoints[scriptHex][wire.OutPoint{Hash: *txHash, Index: uint32(vout)}] = struct{}{}
			a.addMatchLocked(api.TxMatch{
				TxIDHex:      txHash.String(),
				BlockHashHex: blockHash.String(),
				Height:       height,
				Kind:         api.MatchKindOutput,
				Vout:         uint32(vout),
			})
		}
		for vin, input := range msgTx.TxIn {
			if _, ok := a.outpoints[scriptHex][input.PreviousOutPoint]; !ok {
				continue
			}
			a.addMatchLocked(api.TxMatch{
				TxIDHex:      txHash.String(),
				BlockHashHex: blockHash.String(),
				Height:       height,
				Kind:         api.MatchKindSpend,
				Vin:          uint32(vin),
			})
		}
	}
}

func (a *adapter) addMatchLocked(match api.TxMatch) {
	key := fmt.Sprintf("%s:%s:%d:%s:%d:%d", match.TxIDHex, match.BlockHashHex, match.Height, match.Kind, match.Vout, match.Vin)
	if _, ok := a.seen[key]; ok {
		return
	}
	a.seen[key] = struct{}{}
	a.matches = append(a.matches, match)
}

func p2wpkhAddress(script []byte) (btcutil.Address, error) {
	if len(script) != 22 || script[0] != 0x00 || script[1] != 0x14 {
		return nil, fmt.Errorf("neutrino adapter currently accepts P2WPKH scripts only")
	}
	return btcutil.NewAddressWitnessPubKeyHash(script[2:], &chaincfg.RegressionNetParams)
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func main() {
	var listen string
	flag.StringVar(&listen, "listen", "127.0.0.1:0", "HTTP listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("listening=http://%s\n", listener.Addr())
	server := &http.Server{Handler: newAdapter().routes()}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}
