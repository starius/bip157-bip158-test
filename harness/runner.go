package harness

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/bip157-bip158-test/suite/peerlab"
	"github.com/bip157-bip158-test/suite/scenario"
	"github.com/bip157-bip158-test/suite/score"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// Options configures one black-box conformance run.
type Options struct {
	AdapterURL string
	DataDir    string
	Timeout    time.Duration
}

// Run executes the currently implemented scenario set and marks the rest of
// the catalog as skipped. This makes incremental development honest: missing
// scenarios are visible in every report.
func Run(ctx context.Context, opts Options) (score.Summary, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		return score.Summary{}, err
	}

	results := catalogAsSkipped()
	upsert(&results, runBIP158Internal(fixture)...)

	if opts.AdapterURL != "" {
		longFixture, err := chainlab.BuildLongWalletFixture(chainlab.DefaultLongChainHeight)
		if err != nil {
			return score.Summary{}, err
		}
		for _, adapterScenario := range []struct {
			id      string
			title   string
			level   score.Level
			fixture *chainlab.Fixture
			run     func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error)
		}{
			{
				id:      "adapter.honest_wallet_receive_spend",
				title:   "honest peer wallet receive and spend",
				level:   score.Must,
				fixture: longFixture,
				run:     runHonestAdapter,
			},
			{
				id:      "kyoto.various_client_methods",
				title:   "client best block, header, peer, and unknown hash APIs",
				level:   score.Must,
				fixture: longFixture,
				run:     runClientMethodsAdapter,
			},
			{
				id:      "neutrino.sync_without_headers_import.initial_sync",
				title:   "multi-peer initial sync",
				level:   score.Must,
				fixture: longFixture,
				run:     runMultiPeerInitialSyncAdapter,
			},
			{
				id:      "chain.long_checkpointed_header_sync",
				title:   "long chain crosses compact-filter checkpoints",
				level:   score.Must,
				fixture: longFixture,
				run:     runLongChainAdapter,
			},
			{
				id:      "bip157.large_batch_progress_timeout",
				title:   "large compact-filter batches make progress",
				level:   score.Must,
				fixture: longFixture,
				run:     runLargeBatchProgressAdapter,
			},
			{
				id:      "bip157.cfheaders_order_and_checkpoint_boundaries",
				title:   "cfheaders ordering and checkpoint boundaries are handled",
				level:   score.Must,
				fixture: longFixture,
				run:     runCFHeadersBoundaryAdapter,
			},
			{
				id:      "bip157.conflict_one_honest_one_liar",
				title:   "one honest and one liar filter-header conflict",
				level:   score.Should,
				fixture: longFixture,
				run:     runFilterHeaderConflictAdapter,
			},
			{
				id:      "bip157.direct_bad_cfilter_ban",
				title:   "bad direct cfilter response is punished",
				level:   score.Should,
				fixture: longFixture,
				run:     runBadCFilterAdapter,
			},
			{
				id:      "bip157.bad_cfcheckpt_response",
				title:   "bad compact-filter checkpoint response is rejected or punished",
				level:   score.Should,
				fixture: longFixture,
				run:     runBadCFCheckptAdapter,
			},
			{
				id:      "bip157.bad_cfheaders_prev_header",
				title:   "bad compact-filter previous header is rejected or punished",
				level:   score.Should,
				fixture: longFixture,
				run:     runBadPrevFilterHeaderAdapter,
			},
			{
				id:      "bip157.wrong_filter_type_response",
				title:   "wrong filter type responses are rejected or punished",
				level:   score.Should,
				fixture: longFixture,
				run:     runWrongFilterTypeAdapter,
			},
			{
				id:      "neutrino.detect_bad_peers.unresponsive_peer",
				title:   "detect bad peers: unresponsive_peer",
				level:   score.Should,
				fixture: longFixture,
				run:     runUnresponsivePeerAdapter,
			},
			{
				id:      "network.outage_filter_headers",
				title:   "temporary outage during filter-header sync recovers",
				level:   score.Must,
				fixture: longFixture,
				run:     runFilterHeaderOutageAdapter,
			},
			{
				id:      "network.outage_block_download",
				title:   "temporary outage during block download recovers",
				level:   score.Must,
				fixture: longFixture,
				run:     runBlockDownloadOutageAdapter,
			},
		} {
			scenarioOpts := scenarioOptions(opts, adapterScenario.id)
			adapterResults, err := adapterScenario.run(ctx, scenarioOpts, adapterScenario.fixture)
			if err != nil {
				upsert(&results, score.Result{
					ID:       adapterScenario.id,
					Title:    adapterScenario.title,
					Level:    adapterScenario.level,
					Status:   score.Fail,
					Evidence: err.Error(),
				})
			} else {
				upsert(&results, adapterResults...)
			}
		}
	}

	return score.Summarize(results), nil
}

func scenarioOptions(opts Options, id string) Options {
	if opts.DataDir == "" {
		return opts
	}
	opts.DataDir = filepath.Join(opts.DataDir, safePathID(id))
	return opts
}

func safePathID(id string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(id)
}

func catalogAsSkipped() []score.Result {
	defs := scenario.Catalog()
	results := make([]score.Result, 0, len(defs)+1)
	for _, def := range defs {
		results = append(results, score.Result{
			ID:       def.ID,
			Title:    def.Title,
			Level:    def.Level,
			Status:   score.Skipped,
			Evidence: "cataloged but not executed by this harness build",
		})
	}
	results = append(results, score.Result{
		ID:       "adapter.honest_wallet_receive_spend",
		Title:    "honest peer wallet receive and spend",
		Level:    score.Must,
		Status:   score.Skipped,
		Evidence: "requires --adapter-url",
	})
	return results
}

func runBIP158Internal(fixture *chainlab.Fixture) []score.Result {
	var results []score.Result
	coinbase := fixture.Blocks[0]
	coinbaseScript := coinbase.Block.Transactions[0].TxOut[0].PkScript
	matches, err := chainlab.Contains(coinbase.Filter.FilterBytes, coinbase.Block.BlockHash(), coinbaseScript)
	results = append(results, resultFromBool("bip158.coinbase_output_included", score.Must, matches && err == nil, err))

	coinbaseInput, err := chainlab.Contains(
		coinbase.Filter.FilterBytes,
		coinbase.Block.BlockHash(),
		coinbase.Block.Transactions[0].TxIn[0].SignatureScript,
	)
	results = append(results, resultFromBool("bip158.coinbase_input_excluded", score.Must, !coinbaseInput && err == nil, err))

	legacy, err := prevoutScriptIncluded([]byte{
		0x76, 0xa9, 0x14,
		0x22, 0x22, 0x22, 0x22, 0x22,
		0x22, 0x22, 0x22, 0x22, 0x22,
		0x22, 0x22, 0x22, 0x22, 0x22,
		0x22, 0x22, 0x22, 0x22, 0x22,
		0x88, 0xac,
	})
	results = append(results, resultFromBool("bip158.prevout_legacy_included", score.Must, legacy && err == nil, err))

	p2sh, err := prevoutScriptIncluded(append([]byte{0xa9, 0x14}, append(bytesOf(0x55, 20), 0x87)...))
	results = append(results, resultFromBool("bip158.prevout_p2sh_included", score.Must, p2sh && err == nil, err))

	p2wsh, err := prevoutScriptIncluded(append([]byte{0x00, 0x20}, bytesOf(0x66, 32)...))
	results = append(results, resultFromBool("bip158.prevout_p2wsh_included", score.Must, p2wsh && err == nil, err))

	taproot, err := prevoutScriptIncluded(append([]byte{0x51, 0x20}, bytesOf(0x33, 32)...))
	results = append(results, resultFromBool("bip158.prevout_taproot_included", score.Must, taproot && err == nil, err))

	zero, err := zeroElementOPReturnCheck()
	results = append(results, resultFromBool("bip158.zero_element_serialization", score.Must, zero && err == nil, err))
	results = append(results, resultFromBool("bip158.op_return_excluded", score.Must, zero && err == nil, err))
	results = append(results, resultFromBool("bip158.empty_filter_wire_forms", score.Must, zero && err == nil, err))

	fullScript, err := fullScriptNotPushDataCheck()
	results = append(results, resultFromBool("bip158.full_script_not_pushdata", score.Must, fullScript && err == nil, err))
	return results
}

func prevoutScriptIncluded(script []byte) (bool, error) {
	tx := &wire.MsgTx{
		Version: 2,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Index: 0},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []*wire.TxOut{{
			Value:    1,
			PkScript: []byte{0x51},
		}},
	}
	block, err := chainlab.MineFixtureBlock(1, chainhash.Hash{}, []*wire.MsgTx{tx})
	if err != nil {
		return false, err
	}
	material, err := chainlab.BuildFilterMaterial(block, [][]byte{script}, chainhash.Hash{})
	if err != nil {
		return false, err
	}
	return chainlab.Contains(material.FilterBytes, block.BlockHash(), script)
}

func fullScriptNotPushDataCheck() (bool, error) {
	pushed := bytesOf(0x44, 20)
	script := append([]byte{0x00, 0x14}, pushed...)
	serialized, blockHash, err := chainlab.BuildSingleOutputFilter(script)
	if err != nil {
		return false, err
	}
	fullMatch, err := chainlab.Contains(serialized, blockHash, script)
	if err != nil {
		return false, err
	}
	pushOnlyMatch, err := chainlab.Contains(serialized, blockHash, pushed)
	if err != nil {
		return false, err
	}
	return fullMatch && !pushOnlyMatch, nil
}

func zeroElementOPReturnCheck() (bool, error) {
	opReturn := []byte{0x6a, 0x01, 0x01}
	serialized, blockHash, err := chainlab.BuildSingleOutputFilter(opReturn)
	if err != nil {
		return false, err
	}
	matches, err := chainlab.Contains(serialized, blockHash, opReturn)
	if err != nil {
		return false, err
	}
	return len(serialized) == 1 && serialized[0] == 0 && !matches, nil
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func resultFromBool(id string, level score.Level, ok bool, err error) score.Result {
	status := score.Pass
	evidence := "passed"
	if !ok {
		status = score.Fail
		evidence = "failed"
	}
	if err != nil {
		evidence = err.Error()
	}
	return score.Result{ID: id, Level: level, Status: status, Evidence: evidence}
}

func runHonestAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := server.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	req := api.ConfigureRequest{
		Network: "regtest",
		DataDir: opts.DataDir,
		Peers: []api.PeerConfig{{
			ID:      "honest-a",
			Address: server.Addr(),
			Trusted: true,
		}},
		RequiredPeers:  1,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return nil, fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	watch := api.WatchScriptRequest{
		ScriptPubKeyHex: hex.EncodeToString(fixture.WatchedScript),
		StartHeight:     0,
	}
	if err := client.PostJSON(ctx, "/watch-script", watch, nil); err != nil {
		return nil, fmt.Errorf("watch script: %w", err)
	}

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("honest-a", server))
	}

	matchReq := api.GetMatchesRequest{
		ScriptPubKeyHex: hex.EncodeToString(fixture.WatchedScript),
		StartHeight:     0,
		StopHeight:      tip.Height,
	}
	matches, err := waitForMatches(waitCtx, client, matchReq, len(fixture.Matches))
	if err != nil {
		return nil, err
	}

	return []score.Result{{
		ID:       "adapter.honest_wallet_receive_spend",
		Title:    "honest peer wallet receive and spend",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("reported %d matches", len(matches.Matches)),
	}, {
		ID:       "neutrino.sync_without_headers_import.one_shot_rescan",
		Title:    "one-shot rescan",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("reported %d matches from a start-height-0 watch", len(matches.Matches)),
	}, {
		ID:       "neutrino.sync_without_headers_import.long_rescan_start",
		Title:    "long-running rescan start",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("rescan started at height 0 and reached height %d", tip.Height),
	}, {
		ID:       "neutrino.sync_without_headers_import.rescan_results",
		Title:    "long-running rescan results",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("reported %d receive/spend matches", len(matches.Matches)),
	}}, nil
}

func runClientMethodsAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := server.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "client-methods",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("client-methods", server))
	}

	if err := requireKnownBlockHashes(ctx, client, fixture, []uint32{0, 1, 2, 1000, tip.Height}); err != nil {
		return clientMethodFailure(err.Error()), nil
	}
	if err := requireUnknownHeightRejected(ctx, client, tip.Height+1); err != nil {
		return clientMethodFailure(err.Error()), nil
	}
	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return clientMethodFailure(fmt.Sprintf("list peers: %v", err)), nil
	}
	if len(peers.Peers) != 1 || peers.Peers[0].ID != "client-methods" || !peers.Peers[0].Connected {
		return clientMethodFailure(fmt.Sprintf("unexpected peer state: %+v", peers.Peers)), nil
	}

	evidence := fmt.Sprintf("best block, five block-hash lookups, unknown-height rejection, and one whitelisted peer passed at height %d", tip.Height)
	return []score.Result{{
		ID:       "kyoto.various_client_methods",
		Title:    "client best block, header, peer, and unknown hash APIs",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: evidence,
	}, {
		ID:       "kyoto.whitelist_only_sync",
		Title:    "whitelist-only peer selection",
		Level:    score.Should,
		Status:   score.Pass,
		Evidence: "adapter synced with discovery disabled and reported only the configured peer",
	}}, nil
}

func clientMethodFailure(evidence string) []score.Result {
	return []score.Result{{
		ID:       "kyoto.various_client_methods",
		Title:    "client best block, header, peer, and unknown hash APIs",
		Level:    score.Must,
		Status:   score.Fail,
		Evidence: evidence,
	}, {
		ID:       "kyoto.whitelist_only_sync",
		Title:    "whitelist-only peer selection",
		Level:    score.Should,
		Status:   score.Fail,
		Evidence: evidence,
	}}
}

func runMultiPeerInitialSyncAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	peerA := peerlab.NewServer(fixture)
	if err := peerA.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer peerA.Stop()
	peerB := peerlab.NewServer(fixture)
	if err := peerB.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer peerB.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "initial-sync-a",
		Address: peerA.Addr(),
	}, {
		ID:      "initial-sync-b",
		Address: peerB.Addr(),
	}}, 2); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s; %s", err, transcriptSummary("initial-sync-a", peerA), transcriptSummary("initial-sync-b", peerB))
	}
	return []score.Result{{
		ID:       "neutrino.sync_without_headers_import.initial_sync",
		Title:    "multi-peer initial sync",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("adapter reached height %d with two required peers; a: %s; b: %s", tip.Height, transcriptCounts(peerA), transcriptCounts(peerB)),
	}}, nil
}

func runLongChainAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := server.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "long-chain",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("long-chain", server))
	}
	return []score.Result{{
		ID:       "chain.long_checkpointed_header_sync",
		Title:    "long chain crosses compact-filter checkpoints",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("adapter reached long-chain tip height %d; %s", tip.Height, transcriptCounts(server)),
	}}, nil
}

func runLargeBatchProgressAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		DelayOnceByCommand: map[string]time.Duration{"cfheaders": recoverableDelay(opts)},
	}))
	if err := server.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "large-batch",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("large-batch", server))
	}
	return []score.Result{{
		ID:       "bip157.large_batch_progress_timeout",
		Title:    "large compact-filter batches make progress",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("adapter reached height %d after delayed large-batch response; %s", tip.Height, transcriptCounts(server)),
	}}, nil
}

func runCFHeadersBoundaryAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := server.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "cfheaders-boundary",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("cfheaders-boundary", server))
	}
	return []score.Result{{
		ID:       "bip157.cfheaders_order_and_checkpoint_boundaries",
		Title:    "cfheaders ordering and checkpoint boundaries are handled",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("adapter reached height %d across checkpoint boundaries; %s", tip.Height, transcriptCounts(server)),
	}}, nil
}

func runFilterHeaderConflictAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	honest := peerlab.NewServer(fixture)
	if err := honest.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer honest.Stop()

	liar := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFHeaders: map[uint32]bool{2: true},
	}))
	if err := liar.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer liar.Stop()

	client := api.NewClient(opts.AdapterURL)
	req := api.ConfigureRequest{
		Network: "regtest",
		DataDir: opts.DataDir,
		Peers: []api.PeerConfig{{
			ID:      "honest-cfheaders",
			Address: honest.Addr(),
		}, {
			ID:      "liar-cfheaders",
			Address: liar.Addr(),
		}},
		RequiredPeers:  2,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return nil, fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "liar-cfheaders"); ok {
		return filterHeaderConflictResults(score.Pass, evidence), nil
	}
	evidence := "liar-cfheaders was not punished after conflicting filter headers"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s; %s", waitErr, transcriptSummary("honest-cfheaders", honest), transcriptSummary("liar-cfheaders", liar))
	}
	return filterHeaderConflictResults(score.Fail, evidence), nil
}

func filterHeaderConflictResults(status score.Status, evidence string) []score.Result {
	return []score.Result{{
		ID:       "bip157.conflict_one_honest_one_liar",
		Title:    "one honest and one liar filter-header conflict",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}, {
		ID:       "neutrino.cfheaders_mismatch.case_1",
		Title:    "compact-filter-header mismatch case 1",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}}
}

func runBadCFCheckptAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFCheckpts: map[uint32]bool{1000: true},
	}))
	if err := bad.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-cfcheckpt",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "bad-cfcheckpt"); ok {
		return badCFCheckptResults(score.Pass, evidence), nil
	}
	evidence := "bad-cfcheckpt was not punished after serving a corrupt checkpoint"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-cfcheckpt", bad))
	}
	return badCFCheckptResults(score.Fail, evidence), nil
}

func badCFCheckptResults(status score.Status, evidence string) []score.Result {
	return []score.Result{{
		ID:       "bip157.bad_cfcheckpt_response",
		Title:    "bad compact-filter checkpoint response is rejected or punished",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}, {
		ID:       "neutrino.cfcheckpt_sanity.case_1",
		Title:    "compact-filter checkpoint sanity case 1",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}}
}

func runBadPrevFilterHeaderAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptPrevFilterHeader: map[uint32]bool{0: true, 1: true, wire.CFCheckptInterval: true},
	}))
	if err := bad.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-prev-filter-header",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "bad-prev-filter-header"); ok {
		return badPrevFilterHeaderResults(score.Pass, evidence), nil
	}
	evidence := "bad-prev-filter-header was not punished after serving a corrupt previous filter header"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-prev-filter-header", bad))
	}
	return badPrevFilterHeaderResults(score.Fail, evidence), nil
}

func badPrevFilterHeaderResults(status score.Status, evidence string) []score.Result {
	return []score.Result{{
		ID:       "bip157.bad_cfheaders_prev_header",
		Title:    "bad compact-filter previous header is rejected or punished",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}, {
		ID:       "neutrino.blockmanager_invalid_interval.invalid_prev_header",
		Title:    "invalid filter-header interval invalid_prev_header",
		Level:    score.Must,
		Status:   status,
		Evidence: evidence,
	}}
}

func runWrongFilterTypeAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		WrongFilterType: map[string]wire.FilterType{
			"cfilter":   99,
			"cfheaders": 99,
		},
	}))
	if err := bad.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-filter-type",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	if err := client.PostJSON(ctx, "/watch-script", api.WatchScriptRequest{
		ScriptPubKeyHex: hex.EncodeToString(fixture.WatchedScript),
		StartHeight:     0,
	}, nil); err != nil {
		return nil, fmt.Errorf("watch script: %w", err)
	}

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "bad-filter-type"); ok {
		return []score.Result{{
			ID:       "bip157.wrong_filter_type_response",
			Title:    "wrong filter type responses are rejected or punished",
			Level:    score.Should,
			Status:   score.Pass,
			Evidence: evidence,
		}}, nil
	}
	if waitErr != nil {
		return nil, fmt.Errorf("%w; %s", waitErr, transcriptSummary("bad-filter-type", bad))
	}
	return nil, fmt.Errorf("bad-filter-type was not punished after wrong filter type responses")
}

func runBadCFilterAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFilters: map[uint32]bool{2: true},
	}))
	if err := bad.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	req := api.ConfigureRequest{
		Network: "regtest",
		DataDir: opts.DataDir,
		Peers: []api.PeerConfig{{
			ID:      "bad-cfilter",
			Address: bad.Addr(),
		}},
		RequiredPeers:  1,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return nil, fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	watch := api.WatchScriptRequest{
		ScriptPubKeyHex: hex.EncodeToString(fixture.WatchedScript),
		StartHeight:     0,
	}
	if err := client.PostJSON(ctx, "/watch-script", watch, nil); err != nil {
		return nil, fmt.Errorf("watch script: %w", err)
	}

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "bad-cfilter"); ok {
		return badCFilterResults(score.Pass, evidence), nil
	}
	evidence := "bad-cfilter was not punished after serving a corrupt filter"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-cfilter", bad))
	}
	return badCFilterResults(score.Fail, evidence), nil
}

func badCFilterResults(status score.Status, evidence string) []score.Result {
	return []score.Result{{
		ID:       "bip157.direct_bad_cfilter_ban",
		Title:    "bad direct cfilter response is punished",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}, {
		ID:       "neutrino.detect_bad_peers.filter_hash_mismatch",
		Title:    "detect bad peers: filter_hash_mismatch",
		Level:    score.Should,
		Status:   status,
		Evidence: evidence,
	}}
}

func runUnresponsivePeerAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		DelayByCommand: map[string]time.Duration{"headers": unresponsiveDelay(opts)},
	}))
	if err := bad.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "unresponsive-peer",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	probeCtx, cancel := context.WithTimeout(ctx, badPeerProbeTimeout(opts))
	defer cancel()
	waitErr := waitForAdapterTip(probeCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunished(peers, "unresponsive-peer"); ok {
		return []score.Result{{
			ID:       "neutrino.detect_bad_peers.unresponsive_peer",
			Title:    "detect bad peers: unresponsive_peer",
			Level:    score.Should,
			Status:   score.Pass,
			Evidence: evidence,
		}}, nil
	}

	evidence := "unresponsive-peer was not punished after a stalled headers response"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("unresponsive-peer", bad))
	}
	return []score.Result{{
		ID:       "neutrino.detect_bad_peers.unresponsive_peer",
		Title:    "detect bad peers: unresponsive_peer",
		Level:    score.Should,
		Status:   score.Fail,
		Evidence: evidence,
	}}, nil
}

func runFilterHeaderOutageAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	peer := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		DelayOnceByCommand: map[string]time.Duration{"cfheaders": recoverableDelay(opts)},
	}))
	if err := peer.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer peer.Stop()

	client := api.NewClient(opts.AdapterURL)
	req := api.ConfigureRequest{
		Network: "regtest",
		DataDir: opts.DataDir,
		Peers: []api.PeerConfig{{
			ID:      "flaky-cfheaders",
			Address: peer.Addr(),
		}},
		RequiredPeers:  1,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return nil, fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("flaky-cfheaders", peer))
	}
	return []score.Result{{
		ID:       "network.outage_filter_headers",
		Title:    "temporary outage during filter-header sync recovers",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: "adapter reached the fixture tip after a one-shot cfheaders delay",
	}}, nil
}

func runBlockDownloadOutageAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	peer := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		DelayOnceByCommand: map[string]time.Duration{"block": recoverableDelay(opts)},
	}))
	if err := peer.Start("127.0.0.1:0"); err != nil {
		return nil, err
	}
	defer peer.Stop()

	client := api.NewClient(opts.AdapterURL)
	req := api.ConfigureRequest{
		Network: "regtest",
		DataDir: opts.DataDir,
		Peers: []api.PeerConfig{{
			ID:      "flaky-block",
			Address: peer.Addr(),
		}},
		RequiredPeers:  1,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return nil, fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return nil, fmt.Errorf("start adapter: %w", err)
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	scriptHex := hex.EncodeToString(fixture.WatchedScript)
	if err := client.PostJSON(ctx, "/watch-script", api.WatchScriptRequest{
		ScriptPubKeyHex: scriptHex,
		StartHeight:     0,
	}, nil); err != nil {
		return nil, fmt.Errorf("watch script: %w", err)
	}

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("flaky-block", peer))
	}

	matches, err := waitForMatches(waitCtx, client, api.GetMatchesRequest{
		ScriptPubKeyHex: scriptHex,
		StartHeight:     0,
		StopHeight:      tip.Height,
	}, len(fixture.Matches))
	if err != nil {
		return nil, fmt.Errorf("%w after block-delay recovery", err)
	}

	return []score.Result{{
		ID:       "network.outage_block_download",
		Title:    "temporary outage during block download recovers",
		Level:    score.Must,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("reported %d matches after a one-shot block delay", len(matches.Matches)),
	}}, nil
}

func configureAndStart(ctx context.Context, client *api.Client, opts Options, peers []api.PeerConfig, requiredPeers uint32) error {
	req := api.ConfigureRequest{
		Network:        "regtest",
		DataDir:        opts.DataDir,
		Peers:          peers,
		RequiredPeers:  requiredPeers,
		AllowDiscovery: false,
	}
	if err := client.PostJSON(ctx, "/configure", req, nil); err != nil {
		return fmt.Errorf("configure adapter: %w", err)
	}
	if err := client.PostJSON(ctx, "/start", map[string]string{}, nil); err != nil {
		return fmt.Errorf("start adapter: %w", err)
	}
	return nil
}

func requireKnownBlockHashes(ctx context.Context, client jsonPoster, fixture *chainlab.Fixture, heights []uint32) error {
	for _, height := range heights {
		if int(height) >= len(fixture.Blocks) {
			return fmt.Errorf("test fixture does not have height %d", height)
		}
		want := fixture.Blocks[height].Block.BlockHash().String()
		var got api.BlockRef
		if err := client.PostJSON(ctx, "/block-hash", api.BlockRef{Height: height}, &got); err != nil {
			return fmt.Errorf("block-hash height %d: %w", height, err)
		}
		if got.Height != height || got.HashHex != want {
			return fmt.Errorf("block-hash height %d = %d %s, want %s", height, got.Height, got.HashHex, want)
		}
	}
	return nil
}

func requireUnknownHeightRejected(ctx context.Context, client jsonPoster, height uint32) error {
	var got api.BlockRef
	if err := client.PostJSON(ctx, "/block-hash", api.BlockRef{Height: height}, &got); err != nil {
		return nil
	}
	return fmt.Errorf("block-hash height %d unexpectedly succeeded with %s", height, got.HashHex)
}

func recoverableDelay(opts Options) time.Duration {
	if opts.Timeout > 2*time.Second {
		return 500 * time.Millisecond
	}
	if opts.Timeout > 200*time.Millisecond {
		return opts.Timeout / 4
	}
	return 50 * time.Millisecond
}

func unresponsiveDelay(opts Options) time.Duration {
	delay := 15 * time.Second
	if opts.Timeout > 0 && opts.Timeout < delay {
		return opts.Timeout + time.Second
	}
	return delay
}

func badPeerProbeTimeout(opts Options) time.Duration {
	if opts.Timeout > 12*time.Second {
		return 12 * time.Second
	}
	if opts.Timeout > 2*time.Second {
		return opts.Timeout / 2
	}
	return opts.Timeout
}

func transcriptSummary(label string, server *peerlab.Server) string {
	entries := server.Transcript()
	if len(entries) == 0 {
		return label + " transcript: empty"
	}
	const maxEntries = 8
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s/%s/%s/%s", label, entry.Dir, entry.Command, entry.Summary))
	}
	return "peer transcript: " + strings.Join(parts, "; ")
}

func transcriptCounts(server *peerlab.Server) string {
	counts := map[string]int{}
	for _, entry := range server.Transcript() {
		counts[entry.Command]++
	}
	if len(counts) == 0 {
		return "no p2p transcript"
	}
	commands := []string{
		"getheaders",
		"headers",
		"getcfcheckpt",
		"cfcheckpt",
		"getcfheaders",
		"cfheaders",
		"getcfilters",
		"cfilter",
		"getdata",
		"block",
	}
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		if counts[command] > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", command, counts[command]))
		}
	}
	if len(parts) == 0 {
		return "no BIP157 transcript entries"
	}
	return strings.Join(parts, " ")
}

func peerPunished(peers api.ListPeersResponse, id string) (bool, string) {
	for _, peer := range peers.Peers {
		if peer.ID != id {
			continue
		}
		if peer.Banned || !peer.Connected || peer.LastError != "" {
			return true, fmt.Sprintf("peer=%s connected=%t banned=%t last_error=%q", peer.ID, peer.Connected, peer.Banned, peer.LastError)
		}
		return false, fmt.Sprintf("peer=%s connected=%t banned=%t last_error=%q", peer.ID, peer.Connected, peer.Banned, peer.LastError)
	}
	return false, "peer not reported"
}

type jsonPoster interface {
	PostJSON(context.Context, string, any, any) error
}

func waitForAdapterTip(ctx context.Context, client jsonPoster, hash string, height uint32) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var best api.BlockRef
		err := client.PostJSON(ctx, "/best-block", map[string]string{}, &best)
		if err == nil && best.HashHex == hash && best.Height == height {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for adapter tip %d %s", height, hash)
		case <-ticker.C:
		}
	}
}

func waitForMatches(ctx context.Context, client jsonPoster, req api.GetMatchesRequest, want int) (api.GetMatchesResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last api.GetMatchesResponse
	for {
		err := client.PostJSON(ctx, "/matches", req, &last)
		if err == nil && len(last.Matches) >= want {
			return last, nil
		}
		select {
		case <-ctx.Done():
			return last, fmt.Errorf("adapter reported %d matches, expected at least %d", len(last.Matches), want)
		case <-ticker.C:
		}
	}
}

func upsert(results *[]score.Result, replacements ...score.Result) {
	index := map[string]int{}
	for i, result := range *results {
		index[result.ID] = i
	}
	for _, replacement := range replacements {
		if i, ok := index[replacement.ID]; ok {
			(*results)[i] = replacement
			continue
		}
		*results = append(*results, replacement)
	}
}
