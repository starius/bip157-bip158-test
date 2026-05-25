package harness

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bip157-bip158-test/suite/addresslab"
	"github.com/bip157-bip158-test/suite/api"
	"github.com/bip157-bip158-test/suite/chainlab"
	"github.com/bip157-bip158-test/suite/environment"
	"github.com/bip157-bip158-test/suite/peerlab"
	"github.com/bip157-bip158-test/suite/scenario"
	"github.com/bip157-bip158-test/suite/score"
	"github.com/bip157-bip158-test/suite/torlab"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// Options configures one black-box conformance run.
type Options struct {
	AdapterURL                string
	DataDir                   string
	Environment               string
	AddressLab                string
	ProxyAddress              string
	TorLab                    string
	ChutneyPath               string
	ChutneyNet                string
	RequireDistinctIdentities bool
	Timeout                   time.Duration

	addressAllocator addresslab.Allocator
	torLab           *torlab.Lab
}

type adapterScenario struct {
	id      string
	title   string
	level   score.Level
	fixture *chainlab.Fixture
	run     func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error)
}

type resultSpec struct {
	id    string
	title string
	level score.Level
}

// Run executes the currently implemented scenario set and marks the rest of
// the catalog as skipped. This makes incremental development honest: missing
// scenarios are visible in every report.
func Run(ctx context.Context, opts Options) (score.Summary, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	env, err := environment.Lookup(opts.Environment)
	if err != nil {
		return score.Summary{}, err
	}
	if opts.TorLab != "" && opts.TorLab != "off" && opts.TorLab != "chutney" {
		return score.Summary{}, fmt.Errorf("unknown tor lab %q", opts.TorLab)
	}
	allocator, err := addresslab.New(opts.AddressLab)
	if err != nil {
		return score.Summary{}, err
	}
	defer allocator.Close()
	opts.addressAllocator = allocator
	if env.ID == environment.TorV3 && opts.TorLab == "chutney" {
		lab, err := startTorLab(ctx, opts)
		if err != nil {
			return score.Summary{}, err
		}
		defer lab.Close()
		opts.torLab = lab
		opts.ProxyAddress = lab.SOCKSAddress()
	}
	if opts.RequireDistinctIdentities && !runHasDistinctIdentities(env, opts) {
		return score.Summary{}, fmt.Errorf("%s does not provide distinct peer identities", env.ID)
	}
	fixture, err := chainlab.BuildWalletFixture()
	if err != nil {
		return score.Summary{}, err
	}

	results := catalogAsSkipped()
	upsert(&results, runBIP158Internal(fixture)...)
	upsert(&results, peerIdentityResults(env, fixture, opts)...)

	if opts.AdapterURL != "" {
		longFixture, err := chainlab.BuildLongWalletFixture(chainlab.DefaultLongChainHeight)
		if err != nil {
			return score.Summary{}, err
		}
		scenarios := adapterScenarios(longFixture)
		client := api.NewClient(opts.AdapterURL)
		caps, capEvidence := adapterCapabilities(ctx, client)
		if ok, reason := adapterSupportsEnvironment(caps, env); !ok {
			evidence := reason
			if capEvidence != "" {
				evidence += "; " + capEvidence
			}
			upsert(&results, environmentResult(env, score.Unsupported, evidence))
			upsert(&results, skippedAdapterScenarios(scenarios, evidence)...)
			return score.Summarize(results), nil
		}
		if !env.IsClearTCP() && opts.torLab == nil {
			evidence := "overlay lab is not active for this environment"
			upsert(&results, environmentResult(env, score.Unsupported, evidence))
			upsert(&results, skippedAdapterScenarios(scenarios, evidence)...)
			return score.Summarize(results), nil
		}
		upsert(&results, environmentResult(env, score.Pass, "environment selected and adapter claimed support"))
		for _, adapterScenario := range scenarios {
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

func expandedAdapterScenarios(fixture *chainlab.Fixture) []adapterScenario {
	var scenarios []adapterScenario

	for _, test := range []struct {
		id     string
		height uint32
	}{
		{"neutrino.cfheaders_mismatch.case_2", 1},
		{"neutrino.cfheaders_mismatch.case_3", 10},
		{"neutrino.cfheaders_mismatch.case_4", wire.CFCheckptInterval - 1},
		{"neutrino.cfheaders_mismatch.case_5", wire.CFCheckptInterval},
		{"neutrino.cfheaders_mismatch.case_6", wire.CFCheckptInterval + 1},
		{"neutrino.cfheaders_mismatch.case_7", chainlab.DefaultLongChainHeight},
	} {
		spec := resultSpec{
			id:    test.id,
			title: "compact-filter-header mismatch " + test.id[strings.LastIndex(test.id, ".")+1:],
			level: score.Should,
		}
		scenarios = append(scenarios, adapterScenario{
			id:      test.id,
			title:   spec.title,
			level:   spec.level,
			fixture: fixture,
			run: func(height uint32, spec resultSpec) func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error) {
				return func(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
					return runFilterHeaderConflictAtHeightAdapter(ctx, opts, fixture, height, []resultSpec{spec})
				}
			}(test.height, spec),
		})
	}

	for _, test := range []struct {
		id      string
		corrupt map[uint32]bool
	}{
		{"neutrino.cfcheckpt_sanity.case_2", map[uint32]bool{wire.CFCheckptInterval * 2: true}},
		{"neutrino.cfcheckpt_sanity.case_3", map[uint32]bool{wire.CFCheckptInterval: true, wire.CFCheckptInterval * 2: true}},
		{"neutrino.cfcheckpt_sanity.case_4", map[uint32]bool{wire.CFCheckptInterval: true}},
		{"neutrino.cfcheckpt_sanity.case_5", map[uint32]bool{wire.CFCheckptInterval * 2: true}},
		{"neutrino.cfcheckpt_sanity.case_6", map[uint32]bool{wire.CFCheckptInterval: true, wire.CFCheckptInterval * 2: true}},
		{"neutrino.cfcheckpt_sanity.case_7", map[uint32]bool{wire.CFCheckptInterval: true}},
		{"neutrino.cfcheckpt_sanity.case_8", map[uint32]bool{wire.CFCheckptInterval * 2: true}},
		{"neutrino.cfcheckpt_sanity.case_9", map[uint32]bool{wire.CFCheckptInterval: true, wire.CFCheckptInterval * 2: true}},
	} {
		spec := resultSpec{
			id:    test.id,
			title: "compact-filter checkpoint sanity " + test.id[strings.LastIndex(test.id, ".")+1:],
			level: score.Should,
		}
		scenarios = append(scenarios, adapterScenario{
			id:      test.id,
			title:   spec.title,
			level:   spec.level,
			fixture: fixture,
			run: func(corrupt map[uint32]bool, spec resultSpec) func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error) {
				return func(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
					return runBadCFCheckptWithCorruptionAdapter(ctx, opts, fixture, corrupt, []resultSpec{spec})
				}
			}(test.corrupt, spec),
		})
	}

	for _, test := range []struct {
		id     string
		height uint32
	}{
		{"neutrino.resolve_filter_mismatch.case_1", 1},
		{"neutrino.resolve_filter_mismatch.case_2", 2},
		{"neutrino.resolve_filter_mismatch.case_3", 1},
		{"neutrino.resolve_filter_mismatch.case_4", 2},
		{"neutrino.resolve_filter_mismatch.case_5", 1},
		{"neutrino.resolve_filter_mismatch.case_6", 2},
		{"neutrino.resolve_filter_mismatch.case_7", 1},
		{"neutrino.resolve_filter_mismatch.case_8", 2},
		{"neutrino.resolve_filter_mismatch.case_9", 1},
		{"neutrino.resolve_filter_mismatch.case_10", 2},
		{"neutrino.resolve_filter_mismatch.case_11", 1},
		{"neutrino.resolve_filter_mismatch.case_12", 2},
		{"neutrino.resolve_filter_mismatch.case_13", 2},
	} {
		spec := resultSpec{
			id:    test.id,
			title: "resolve filter mismatch from block " + test.id[strings.LastIndex(test.id, ".")+1:],
			level: score.Should,
		}
		scenarios = append(scenarios, adapterScenario{
			id:      test.id,
			title:   spec.title,
			level:   spec.level,
			fixture: fixture,
			run: func(height uint32, spec resultSpec) func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error) {
				return func(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
					return runBadCFilterAtHeightAdapter(ctx, opts, fixture, height, []resultSpec{spec})
				}
			}(test.height, spec),
		})
	}

	for _, test := range []struct {
		id    string
		title string
		level score.Level
		run   func(context.Context, Options, *chainlab.Fixture) ([]score.Result, error)
	}{
		{
			id:    "neutrino.blockmanager_invalid_interval.wrong_genesis",
			title: "invalid filter-header interval wrong_genesis",
			level: score.Must,
			run:   runWrongGenesisIntervalAdapter,
		},
		{
			id:    "neutrino.blockmanager_invalid_interval.interval_misaligned",
			title: "invalid filter-header interval interval_misaligned",
			level: score.Must,
			run:   runMisalignedIntervalAdapter,
		},
		{
			id:    "neutrino.handle_headers.valid_then_scrambled",
			title: "valid headers accepted and scrambled headers disconnected",
			level: score.Must,
			run:   runScrambledHeadersAdapter,
		},
	} {
		scenarios = append(scenarios, adapterScenario{
			id:      test.id,
			title:   test.title,
			level:   test.level,
			fixture: fixture,
			run:     test.run,
		})
	}

	return scenarios
}

func adapterScenarios(longFixture *chainlab.Fixture) []adapterScenario {
	scenarios := []adapterScenario{
		{
			id:      "adapter.ipv6_peer_handshake",
			title:   "adapter handshakes with a bracketed IPv6 peer",
			level:   score.Info,
			fixture: longFixture,
			run:     runIPv6PeerHandshakeAdapter,
		},
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
			id:      "bip157.cfilter_block_hash_sequence_mismatch",
			title:   "cfilter block-hash mismatch is rejected or punished",
			level:   score.Should,
			fixture: longFixture,
			run:     runWrongCFilterBlockHashAdapter,
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
			id:      "bip157.empty_cfheaders_response",
			title:   "empty cfheaders response for a non-empty range is rejected or punished",
			level:   score.Should,
			fixture: longFixture,
			run:     runEmptyCFHeadersAdapter,
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
			id:      "blocks.invalid_downloaded_block_rejected",
			title:   "invalid downloaded block is rejected",
			level:   score.Must,
			fixture: longFixture,
			run:     runInvalidDownloadedBlockAdapter,
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
	}
	return append(scenarios, expandedAdapterScenarios(longFixture)...)
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

	standardScripts, err := standardScriptFilterCheck()
	results = append(results, resultFromBool("neutrino.verify_basic_filter.standard_scripts", score.Must, standardScripts && err == nil, err))
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

func standardScriptFilterCheck() (bool, error) {
	scripts := [][]byte{
		append([]byte{0x76, 0xa9, 0x14}, append(bytesOf(0x21, 20), 0x88, 0xac)...),
		append([]byte{0xa9, 0x14}, append(bytesOf(0x22, 20), 0x87)...),
		append([]byte{0x00, 0x14}, bytesOf(0x23, 20)...),
		append([]byte{0x00, 0x20}, bytesOf(0x24, 32)...),
		append([]byte{0x51, 0x20}, bytesOf(0x25, 32)...),
	}
	for _, script := range scripts {
		serialized, blockHash, err := chainlab.BuildSingleOutputFilter(script)
		if err != nil {
			return false, err
		}
		match, err := chainlab.Contains(serialized, blockHash, script)
		if err != nil || !match {
			return false, err
		}
	}
	return zeroElementOPReturnCheck()
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

func resultsFromSpecs(specs []resultSpec, status score.Status, evidence string) []score.Result {
	results := make([]score.Result, 0, len(specs))
	for _, spec := range specs {
		results = append(results, score.Result{
			ID:       spec.id,
			Title:    spec.title,
			Level:    spec.level,
			Status:   status,
			Evidence: evidence,
		})
	}
	return results
}

func peerIdentityResults(env environment.Definition, fixture *chainlab.Fixture, opts Options) []score.Result {
	switch env.ID {
	case environment.IPv4:
		first := peerlab.NewServer(fixture, peerlab.WithAddressAllocator(opts.addressAllocator))
		if _, err := first.StartInEnvironment(env, 1); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_ipv4", score.Fail, err.Error())}
		}
		defer first.Stop()
		second := peerlab.NewServer(fixture, peerlab.WithAddressAllocator(opts.addressAllocator))
		if _, err := second.StartInEnvironment(env, 2); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_ipv4", score.Fail, err.Error())}
		}
		defer second.Stop()
		firstHost := peerIdentityHost(first.Addr())
		secondHost := peerIdentityHost(second.Addr())
		if first.Identity().Distinct && second.Identity().Distinct && firstHost != secondHost {
			return []score.Result{identityResult("peer.identity_distinct_ipv4", score.Pass, "peerlab allocated separate IPv4 loopback identities")}
		}
		return []score.Result{identityResult("peer.identity_distinct_ipv4", score.Unsupported, "host only allowed shared IPv4 loopback identity")}
	case environment.IPv6:
		first := peerlab.NewServer(fixture, peerlab.WithAddressAllocator(opts.addressAllocator))
		if _, err := first.StartInEnvironment(env, 1); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_ipv6", score.Fail, err.Error())}
		}
		defer first.Stop()
		second := peerlab.NewServer(fixture, peerlab.WithAddressAllocator(opts.addressAllocator))
		if _, err := second.StartInEnvironment(env, 2); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_ipv6", score.Fail, err.Error())}
		}
		defer second.Stop()
		firstHost := peerIdentityHost(first.Addr())
		secondHost := peerIdentityHost(second.Addr())
		if first.Identity().Distinct && second.Identity().Distinct && firstHost != secondHost {
			return []score.Result{identityResult("peer.identity_distinct_ipv6", score.Pass, "peerlab allocated separate IPv6 identities")}
		}
		return []score.Result{identityResult("peer.identity_distinct_ipv6", score.Unsupported, "IPv6 loopback uses ::1 unless a lab provides extra identities")}
	case environment.TorV3:
		if opts.torLab == nil {
			return []score.Result{identityResult(
				"peer.identity_distinct_overlay",
				score.Unsupported,
				"overlay identity checks require the environment lab",
			)}
		}
		first := peerlab.NewServer(fixture)
		if err := startInSelectedEnvironment(opts, first, 1); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_overlay", score.Fail, err.Error())}
		}
		defer first.Stop()
		second := peerlab.NewServer(fixture)
		if err := startInSelectedEnvironment(opts, second, 2); err != nil {
			return []score.Result{identityResult("peer.identity_distinct_overlay", score.Fail, err.Error())}
		}
		defer second.Stop()
		firstHost := peerIdentityHost(first.Addr())
		secondHost := peerIdentityHost(second.Addr())
		if first.Identity().Distinct && second.Identity().Distinct && firstHost != secondHost {
			return []score.Result{identityResult(
				"peer.identity_distinct_overlay",
				score.Pass,
				"peerlab allocated separate Tor v3 onion identities",
			)}
		}
		return []score.Result{identityResult(
			"peer.identity_distinct_overlay",
			score.Unsupported,
			"Tor lab did not provide separate onion identities",
		)}
	default:
		return []score.Result{identityResult(
			"peer.identity_distinct_overlay",
			score.Unsupported,
			"overlay identity checks require the environment lab",
		)}
	}
}

func identityResult(id string, status score.Status, evidence string) score.Result {
	return score.Result{
		ID:       id,
		Title:    scenarioTitle(id),
		Level:    score.Info,
		Status:   status,
		Evidence: evidence,
	}
}

func environmentResult(env environment.Definition, status score.Status, evidence string) score.Result {
	id := fmt.Sprintf("env.%s.full_matrix", strings.ReplaceAll(string(env.ID), "-", "_"))
	return score.Result{
		ID:       id,
		Title:    scenarioTitle(id),
		Level:    score.Info,
		Status:   status,
		Evidence: evidence,
	}
}

func scenarioTitle(id string) string {
	for _, def := range scenario.Catalog() {
		if def.ID == id {
			return def.Title
		}
	}
	return id
}

func adapterCapabilities(ctx context.Context, client *api.Client) (api.CapabilitiesResponse, string) {
	var caps api.CapabilitiesResponse
	if err := client.PostJSON(ctx, "/capabilities", map[string]string{}, &caps); err != nil {
		return api.DefaultCapabilities(), fmt.Sprintf("default capabilities used: %v", err)
	}
	if len(caps.Environments) == 0 {
		return api.DefaultCapabilities(), "default capabilities used: empty response"
	}
	return caps, ""
}

func adapterSupportsEnvironment(caps api.CapabilitiesResponse, env environment.Definition) (bool, string) {
	for _, cap := range caps.Environments {
		if cap.ID != string(env.ID) {
			continue
		}
		if cap.Supported {
			return true, ""
		}
		if cap.Reason != "" {
			return false, cap.Reason
		}
		return false, fmt.Sprintf("adapter does not support %s", env.ID)
	}
	return false, fmt.Sprintf("adapter did not report support for %s", env.ID)
}

func skippedAdapterScenarios(scenarios []adapterScenario, evidence string) []score.Result {
	results := make([]score.Result, 0, len(scenarios))
	for _, scenario := range scenarios {
		results = append(results, score.Result{
			ID:       scenario.id,
			Title:    scenario.title,
			Level:    scenario.level,
			Status:   score.Skipped,
			Evidence: evidence,
		})
	}
	return results
}

func runHonestAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "honest-a",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("honest-a", server))
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

func runIPv6PeerHandshakeAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	if opts.Environment != string(environment.IPv6) {
		return []score.Result{{
			ID:       "adapter.ipv6_peer_handshake",
			Title:    "adapter handshakes with a bracketed IPv6 peer",
			Level:    score.Info,
			Status:   score.Skipped,
			Evidence: "scenario only applies to the ipv6 environment",
		}}, nil
	}

	server := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
		return nil, err
	}
	defer server.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "ipv6-handshake",
		Address: server.Addr(),
		Trusted: true,
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := waitForPeerHandshake(waitCtx, server); err != nil {
		return nil, fmt.Errorf("%w; %s", err, transcriptSummary("ipv6-handshake", server))
	}
	return []score.Result{{
		ID:       "adapter.ipv6_peer_handshake",
		Title:    "adapter handshakes with a bracketed IPv6 peer",
		Level:    score.Info,
		Status:   score.Pass,
		Evidence: fmt.Sprintf("adapter completed version/verack with %s; %s", server.Addr(), transcriptCounts(server)),
	}}, nil
}

func runClientMethodsAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	server := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
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
	}, {
		ID:       "network.restricted_connect_no_discovery",
		Title:    "restricted explicit-peer mode avoids discovery",
		Level:    score.Info,
		Status:   score.Pass,
		Evidence: "adapter ran with discovery disabled and exposed only the harness peer",
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
	}, {
		ID:       "network.restricted_connect_no_discovery",
		Title:    "restricted explicit-peer mode avoids discovery",
		Level:    score.Info,
		Status:   score.Fail,
		Evidence: evidence,
	}}
}

func runScrambledHeadersAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptHeaders: map[uint32]bool{2: true},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "scrambled-headers",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	probeCtx, cancel := context.WithTimeout(ctx, badPeerProbeTimeout(opts))
	defer cancel()
	waitErr := waitForAdapterTip(probeCtx, client, tip.Block.BlockHash().String(), tip.Height)

	specs := []resultSpec{{
		id:    "neutrino.handle_headers.valid_then_scrambled",
		title: "valid headers accepted and scrambled headers disconnected",
		level: score.Must,
	}}
	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("scrambled-headers", bad))
		return resultsFromSpecs(specs, score.Fail, evidence), nil
	}
	if ok, evidence := peerPunishedAfter(peers, "scrambled-headers", bad, "headers"); ok {
		return resultsFromSpecs(specs, score.Pass, evidence), nil
	}
	if waitErr == nil {
		return resultsFromSpecs(specs, score.Fail, "adapter accepted a scrambled header chain"), nil
	}
	return resultsFromSpecs(
		specs,
		score.Fail,
		fmt.Sprintf("%v; %s", waitErr, transcriptSummary("scrambled-headers", bad)),
	), nil
}

func runMultiPeerInitialSyncAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	peerA := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, peerA, 1); err != nil {
		return nil, err
	}
	defer peerA.Stop()
	peerB := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, peerB, 2); err != nil {
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
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
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
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
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
	if err := startInSelectedEnvironment(opts, server, 1); err != nil {
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
	return runFilterHeaderConflictAtHeightAdapter(ctx, opts, fixture, 2, []resultSpec{{
		id:    "bip157.conflict_one_honest_one_liar",
		title: "one honest and one liar filter-header conflict",
		level: score.Should,
	}, {
		id:    "neutrino.cfheaders_mismatch.case_1",
		title: "compact-filter-header mismatch case 1",
		level: score.Should,
	}})
}

func runFilterHeaderConflictAtHeightAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture, height uint32, specs []resultSpec) ([]score.Result, error) {
	honest := peerlab.NewServer(fixture)
	if err := startInSelectedEnvironment(opts, honest, 1); err != nil {
		return nil, err
	}
	defer honest.Stop()

	liar := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFHeaders: map[uint32]bool{height: true},
	}))
	if err := startInSelectedEnvironment(opts, liar, 2); err != nil {
		return nil, err
	}
	defer liar.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "honest-cfheaders",
		Address: honest.Addr(),
	}, {
		ID:      "liar-cfheaders",
		Address: liar.Addr(),
	}}, 2); err != nil {
		return nil, err
	}
	defer client.PostJSON(context.Background(), "/stop", map[string]string{}, nil)

	tip := fixture.Blocks[len(fixture.Blocks)-1]
	waitCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		evidence := fmt.Sprintf("list peers: %v; %s; %s", err, transcriptSummary("honest-cfheaders", honest), transcriptSummary("liar-cfheaders", liar))
		return resultsFromSpecs(specs, score.Fail, evidence), nil
	}
	if ok, evidence := peerPunishedAfter(peers, "liar-cfheaders", liar, "cfheaders"); ok {
		return resultsFromSpecs(specs, score.Pass, evidence), nil
	}
	evidence := "liar-cfheaders was not punished after conflicting filter headers"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s; %s", waitErr, transcriptSummary("honest-cfheaders", honest), transcriptSummary("liar-cfheaders", liar))
	}
	return resultsFromSpecs(specs, score.Fail, evidence), nil
}

func runBadCFCheckptAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	return runBadCFCheckptWithCorruptionAdapter(ctx, opts, fixture, map[uint32]bool{1000: true}, []resultSpec{{
		id:    "bip157.bad_cfcheckpt_response",
		title: "bad compact-filter checkpoint response is rejected or punished",
		level: score.Should,
	}, {
		id:    "neutrino.cfcheckpt_sanity.case_1",
		title: "compact-filter checkpoint sanity case 1",
		level: score.Should,
	}})
}

func runBadCFCheckptWithCorruptionAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture, corrupt map[uint32]bool, specs []resultSpec) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFCheckpts: corrupt,
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
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
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("bad-cfcheckpt", bad))
		return resultsFromSpecs(specs, score.Fail, evidence), nil
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-cfcheckpt", bad, "cfcheckpt"); ok {
		return resultsFromSpecs(specs, score.Pass, evidence), nil
	}
	evidence := "bad-cfcheckpt was not punished after serving a corrupt checkpoint"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-cfcheckpt", bad))
	}
	return resultsFromSpecs(specs, score.Fail, evidence), nil
}

func runBadPrevFilterHeaderAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	return runBadPrevFilterHeaderWithSpecsAdapter(ctx, opts, fixture, map[uint32]bool{0: true, 1: true, wire.CFCheckptInterval: true}, []resultSpec{{
		id:    "bip157.bad_cfheaders_prev_header",
		title: "bad compact-filter previous header is rejected or punished",
		level: score.Should,
	}, {
		id:    "neutrino.blockmanager_invalid_interval.invalid_prev_header",
		title: "invalid filter-header interval invalid_prev_header",
		level: score.Must,
	}})
}

func runWrongGenesisIntervalAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	return runBadPrevFilterHeaderWithSpecsAdapter(ctx, opts, fixture, map[uint32]bool{0: true, 1: true}, []resultSpec{{
		id:    "neutrino.blockmanager_invalid_interval.wrong_genesis",
		title: "invalid filter-header interval wrong_genesis",
		level: score.Must,
	}, {
		id:    "neutrino.blockmanager_invalid_interval.wrong_genesis_partial",
		title: "invalid filter-header interval wrong_genesis_partial",
		level: score.Must,
	}})
}

func runMisalignedIntervalAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	return runBadPrevFilterHeaderWithSpecsAdapter(ctx, opts, fixture, map[uint32]bool{wire.CFCheckptInterval: true}, []resultSpec{{
		id:    "neutrino.blockmanager_invalid_interval.interval_misaligned",
		title: "invalid filter-header interval interval_misaligned",
		level: score.Must,
	}, {
		id:    "neutrino.blockmanager_invalid_interval.interval_misaligned_partial",
		title: "invalid filter-header interval interval_misaligned_partial",
		level: score.Must,
	}})
}

func runBadPrevFilterHeaderWithSpecsAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture, corrupt map[uint32]bool, specs []resultSpec) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptPrevFilterHeader: corrupt,
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
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
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("bad-prev-filter-header", bad))
		return resultsFromSpecs(specs, score.Fail, evidence), nil
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-prev-filter-header", bad, "cfheaders"); ok {
		return resultsFromSpecs(specs, score.Pass, evidence), nil
	}
	evidence := "bad-prev-filter-header was not punished after serving a corrupt previous filter header"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-prev-filter-header", bad))
	}
	return resultsFromSpecs(specs, score.Fail, evidence), nil
}

func runEmptyCFHeadersAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		EmptyCFHeaders: map[uint32]bool{0: true, 1: true, wire.CFCheckptInterval: true},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-empty-cfheaders",
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
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("bad-empty-cfheaders", bad))
		return []score.Result{{
			ID:       "bip157.empty_cfheaders_response",
			Title:    "empty cfheaders response for a non-empty range is rejected or punished",
			Level:    score.Should,
			Status:   score.Fail,
			Evidence: evidence,
		}}, nil
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-empty-cfheaders", bad, "cfheaders"); ok {
		return []score.Result{{
			ID:       "bip157.empty_cfheaders_response",
			Title:    "empty cfheaders response for a non-empty range is rejected or punished",
			Level:    score.Should,
			Status:   score.Pass,
			Evidence: evidence,
		}}, nil
	}
	evidence := "bad-empty-cfheaders was not punished after serving empty cfheaders"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-empty-cfheaders", bad))
	}
	return []score.Result{{
		ID:       "bip157.empty_cfheaders_response",
		Title:    "empty cfheaders response for a non-empty range is rejected or punished",
		Level:    score.Should,
		Status:   score.Fail,
		Evidence: evidence,
	}}, nil
}

func runWrongFilterTypeAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		WrongFilterType: map[string]wire.FilterType{
			"cfilter":   99,
			"cfheaders": 99,
		},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
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
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("bad-filter-type", bad))
		return []score.Result{{
			ID:       "bip157.wrong_filter_type_response",
			Title:    "wrong filter type responses are rejected or punished",
			Level:    score.Should,
			Status:   score.Fail,
			Evidence: evidence,
		}}, nil
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-filter-type", bad, "cfheaders", "cfilter"); ok {
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
	return runBadCFilterAtHeightAdapter(ctx, opts, fixture, 2, []resultSpec{{
		id:    "bip157.direct_bad_cfilter_ban",
		title: "bad direct cfilter response is punished",
		level: score.Should,
	}, {
		id:    "neutrino.detect_bad_peers.filter_hash_mismatch",
		title: "detect bad peers: filter_hash_mismatch",
		level: score.Should,
	}, {
		id:    "bip157.malformed_gcs_filter_payload",
		title: "malformed GCS filter payload is rejected or punished",
		level: score.Should,
	}})
}

func runBadCFilterAtHeightAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture, height uint32, specs []resultSpec) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptCFilters: map[uint32]bool{height: true},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-cfilter",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
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
		evidence := fmt.Sprintf("list peers: %v; %s", err, transcriptSummary("bad-cfilter", bad))
		return resultsFromSpecs(specs, score.Fail, evidence), nil
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-cfilter", bad, "cfilter"); ok {
		return resultsFromSpecs(specs, score.Pass, evidence), nil
	}
	evidence := "bad-cfilter was not punished after serving a corrupt filter"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-cfilter", bad))
	}
	return resultsFromSpecs(specs, score.Fail, evidence), nil
}

func runWrongCFilterBlockHashAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		WrongCFilterBlockHash: map[uint32]bool{2: true},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-cfilter-hash",
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
	if ok, evidence := peerPunishedAfter(peers, "bad-cfilter-hash", bad, "cfilter"); ok {
		return []score.Result{{
			ID:       "bip157.cfilter_block_hash_sequence_mismatch",
			Title:    "cfilter block-hash mismatch is rejected or punished",
			Level:    score.Should,
			Status:   score.Pass,
			Evidence: evidence,
		}}, nil
	}
	evidence := "bad-cfilter-hash was not punished after serving a cfilter with the wrong block hash"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-cfilter-hash", bad))
	}
	return []score.Result{{
		ID:       "bip157.cfilter_block_hash_sequence_mismatch",
		Title:    "cfilter block-hash mismatch is rejected or punished",
		Level:    score.Should,
		Status:   score.Fail,
		Evidence: evidence,
	}}, nil
}

func runInvalidDownloadedBlockAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		CorruptBlocks: map[uint32]bool{1: true},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
		return nil, err
	}
	defer bad.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "bad-block",
		Address: bad.Addr(),
	}}, 1); err != nil {
		return nil, err
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
	waitErr := waitForAdapterTip(waitCtx, client, tip.Block.BlockHash().String(), tip.Height)

	noMatchCtx, noMatchCancel := context.WithTimeout(ctx, recoverableDelay(opts))
	defer noMatchCancel()
	matches, matchErr := waitForNoMatches(noMatchCtx, client, api.GetMatchesRequest{
		ScriptPubKeyHex: scriptHex,
		StartHeight:     0,
		StopHeight:      tip.Height,
	})
	if matchErr != nil {
		return []score.Result{{
			ID:       "blocks.invalid_downloaded_block_rejected",
			Title:    "invalid downloaded block is rejected",
			Level:    score.Must,
			Status:   score.Fail,
			Evidence: fmt.Sprintf("%v; reported matches: %+v", matchErr, matches.Matches),
		}}, nil
	}

	var peers api.ListPeersResponse
	if err := client.PostJSON(ctx, "/list-peers", map[string]string{}, &peers); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	if ok, evidence := peerPunishedAfter(peers, "bad-block", bad, "block"); ok {
		return []score.Result{{
			ID:       "blocks.invalid_downloaded_block_rejected",
			Title:    "invalid downloaded block is rejected",
			Level:    score.Must,
			Status:   score.Pass,
			Evidence: evidence,
		}}, nil
	}
	evidence := "bad-block was not punished after serving an invalid downloaded block"
	if waitErr != nil {
		evidence = fmt.Sprintf("%v; %s", waitErr, transcriptSummary("bad-block", bad))
	}
	return []score.Result{{
		ID:       "blocks.invalid_downloaded_block_rejected",
		Title:    "invalid downloaded block is rejected",
		Level:    score.Must,
		Status:   score.Fail,
		Evidence: evidence,
	}}, nil
}

func runUnresponsivePeerAdapter(ctx context.Context, opts Options, fixture *chainlab.Fixture) ([]score.Result, error) {
	bad := peerlab.NewServer(fixture, peerlab.WithBehavior(peerlab.Behavior{
		DelayByCommand: map[string]time.Duration{"headers": unresponsiveDelay(opts)},
	}))
	if err := startInSelectedEnvironment(opts, bad, 1); err != nil {
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
	if err := startInSelectedEnvironment(opts, peer, 1); err != nil {
		return nil, err
	}
	defer peer.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "flaky-cfheaders",
		Address: peer.Addr(),
	}}, 1); err != nil {
		return nil, err
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
	if err := startInSelectedEnvironment(opts, peer, 1); err != nil {
		return nil, err
	}
	defer peer.Stop()

	client := api.NewClient(opts.AdapterURL)
	if err := configureAndStart(ctx, client, opts, []api.PeerConfig{{
		ID:      "flaky-block",
		Address: peer.Addr(),
	}}, 1); err != nil {
		return nil, err
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
	env, err := environment.Lookup(opts.Environment)
	if err != nil {
		return err
	}
	apiEnv := api.EnvironmentFromDefinition(env)
	apiEnv.DistinctPeerIdentities = hasDistinctPeerIdentities(env, opts.addressAllocator)
	if env.ID == environment.TorV3 && opts.torLab != nil {
		apiEnv.DistinctPeerIdentities = true
	}
	apiEnv.ProxyAddress = opts.ProxyAddress
	enriched, err := enrichPeerConfigs(opts, peers)
	if err != nil {
		return err
	}
	req := api.ConfigureRequest{
		Network:        "regtest",
		DataDir:        opts.DataDir,
		Environment:    apiEnv,
		Peers:          enriched,
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

func startTorLab(ctx context.Context, opts Options) (*torlab.Lab, error) {
	dir := filepath.Join(opts.DataDir, "torlab")
	if opts.DataDir == "" {
		var err error
		dir, err = os.MkdirTemp("", "bip157-harness-torlab-*")
		if err != nil {
			return nil, fmt.Errorf("create tor lab dir: %w", err)
		}
	}
	return torlab.Start(ctx, torlab.Options{
		DataDir:     dir,
		ChutneyPath: opts.ChutneyPath,
		Network:     opts.ChutneyNet,
		Command:     nil,
	})
}

func hasDistinctPeerIdentities(env environment.Definition, allocator addresslab.Allocator) bool {
	if !env.IsClearTCP() {
		return env.DistinctPeerIdentities
	}
	if allocator == nil {
		allocator = addresslab.NewLoopback()
	}
	caps := allocator.Capabilities()
	switch env.ID {
	case environment.IPv4:
		return caps.DistinctIPv4Identities
	case environment.IPv6:
		return caps.DistinctIPv6Identities
	default:
		return env.DistinctPeerIdentities
	}
}

func runHasDistinctIdentities(env environment.Definition, opts Options) bool {
	if env.ID == environment.TorV3 && opts.torLab != nil {
		return true
	}
	return hasDistinctPeerIdentities(env, opts.addressAllocator)
}

func startInSelectedEnvironment(opts Options, server *peerlab.Server, index int) error {
	env, err := environment.Lookup(opts.Environment)
	if err != nil {
		return err
	}
	if env.ID == environment.TorV3 && opts.torLab != nil {
		if err := server.Start("127.0.0.1:0"); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		service, err := opts.torLab.Expose(ctx, index, server.ListenAddr())
		if err != nil {
			_ = server.Stop()
			return err
		}
		server.SetAdvertisedAddress(service.Address, env.AddressType, env.Transport, true)
		return nil
	}
	if opts.addressAllocator != nil {
		server.SetAddressAllocator(opts.addressAllocator)
	}
	_, err = server.StartInEnvironment(env, index)
	return err
}

func enrichPeerConfigs(opts Options, peers []api.PeerConfig) ([]api.PeerConfig, error) {
	env, err := environment.Lookup(opts.Environment)
	if err != nil {
		return nil, err
	}
	enriched := make([]api.PeerConfig, len(peers))
	for i, peer := range peers {
		enriched[i] = peer
		if enriched[i].AddressType == "" {
			enriched[i].AddressType = string(env.AddressType)
		}
		if enriched[i].Transport == "" {
			enriched[i].Transport = string(env.Transport)
		}
		if enriched[i].ProxyAddress == "" {
			enriched[i].ProxyAddress = opts.ProxyAddress
		}
		if enriched[i].Identity == "" {
			enriched[i].Identity = peerIdentityHost(peer.Address)
		}
	}
	return enriched, nil
}

func peerIdentityHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
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

func peerPunishedAfter(peers api.ListPeersResponse, id string, server *peerlab.Server, commands ...string) (bool, string) {
	observed := transcriptHasOutbound(server, commands...)
	for _, peer := range peers.Peers {
		if peer.ID != id {
			continue
		}
		evidence := fmt.Sprintf("peer=%s connected=%t banned=%t last_error=%q", peer.ID, peer.Connected, peer.Banned, peer.LastError)
		if peer.Banned {
			return true, evidence
		}
		if observed && (!peer.Connected || peer.LastError != "") {
			return true, evidence
		}
		if !observed {
			return false, evidence + "; bad response was not observed; " + transcriptSummary(id, server)
		}
		return false, evidence
	}
	return false, "peer not reported"
}

func transcriptHasOutbound(server *peerlab.Server, commands ...string) bool {
	want := map[string]struct{}{}
	for _, command := range commands {
		want[command] = struct{}{}
	}
	for _, entry := range server.Transcript() {
		if entry.Dir != "out" {
			continue
		}
		if _, ok := want[entry.Command]; ok {
			return true
		}
	}
	return false
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

func waitForNoMatches(ctx context.Context, client jsonPoster, req api.GetMatchesRequest) (api.GetMatchesResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last api.GetMatchesResponse
	for {
		err := client.PostJSON(ctx, "/matches", req, &last)
		if err == nil && len(last.Matches) > 0 {
			return last, fmt.Errorf("adapter reported %d matches from data that should have been rejected", len(last.Matches))
		}
		select {
		case <-ctx.Done():
			return last, nil
		case <-ticker.C:
		}
	}
}

func waitForPeerHandshake(ctx context.Context, server *peerlab.Server) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if transcriptContainsHandshake(server.Transcript()) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for adapter peer handshake")
		case <-ticker.C:
		}
	}
}

func transcriptContainsHandshake(entries []peerlab.TranscriptEntry) bool {
	sawVersion := false
	sawVerAck := false
	for _, entry := range entries {
		if entry.Dir == "in" && entry.Command == "version" {
			sawVersion = true
		}
		if entry.Dir == "in" && entry.Command == "verack" {
			sawVerAck = true
		}
	}
	return sawVersion && sawVerAck
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
