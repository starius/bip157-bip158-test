# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-23

This is the current execution plan for `/home/user/bip157-bip158-test`.
Completed bootstrap work and completed long-chain work have been removed from
the active plan. The remaining items focus on correctness gaps that still need
implementation or deeper investigation.

## Goal

Build a reproducible regtest conformance suite for BIP157/BIP158 light-client
implementations.

The suite validates an implementation through a small adapter binary. The
adapter exposes a fixed API, connects only to harness-controlled regtest peers,
and reports best block, watched-script matches, block hashes by height, and
peer state where available. The harness creates deterministic chains, honest
peers, adversarial peers, temporary network failures, and conformance reports.

Score meanings:

- `green`: all executable `MUST` and selected `SHOULD` scenarios pass.
- `orange`: all executable `MUST` scenarios pass, but one or more executable
  `SHOULD` scenarios fail or are unsupported.
- `red`: any executable `MUST` scenario fails, the adapter crashes, mandatory
  wallet activity is missed, invalid mandatory data is accepted, or ordinary
  temporary faults require manual restart or data deletion.

Cataloged-but-unimplemented scenarios are reported as `skipped`. A skip does
not count as success.

## Current Built State

The suite already has:

- A fixed adapter schema in `proto/bip157test.proto` and HTTP/JSON API types in
  `api/`.
- Deterministic short and long regtest fixtures in `chainlab/`; the long
  fixture reaches height 2005 and crosses two compact-filter checkpoint
  intervals.
- Independent BIP158 checks for coinbase outputs, coinbase input exclusion,
  OP_RETURN exclusion, zero-element serialization, full-script matching, and
  legacy, P2SH, P2WSH, and Taproot prevout scripts.
- A Bitcoin P2P simulator in `peerlab/` that can serve headers, blocks,
  `cfheaders`, `cfilters`, `cfcheckpt`, paginate long headers, corrupt selected
  compact-filter data, corrupt `PrevFilterHeader`, return wrong filter types,
  delay selected responses, and record transcripts.
- A harness in `harness/` and `cmd/bip157-harness`.
- A reference fake adapter in `cmd/fake-adapter`.
- Kyoto and Neutrino adapters in `adapters/kyoto` and `adapters/neutrino`.
  Nakamoto and Wasabi are planned adapter/research targets, described below.
- Scenario metadata in `scenario/catalog.go` that includes the existing Kyoto
  and Neutrino baseline tests plus stronger conformance scenarios.

Latest recorded validation is in `VALIDATION_REPORT.md`:

- Fake adapter: green.
- Kyoto adapter: red because the malformed `cfheaders` previous-header scenario
  is a mandatory failure; several SHOULD adversarial peer checks also fail.
- Neutrino adapter: red because it disconnects after peerlab's first 2000-header
  page and does not reach compact-filter sync.

## Nakamoto and Wasabi Adapter Decision

Nakamoto is a strict adapter target. The current `cloudhead/nakamoto` codebase
is a Rust light client with native P2P BIP157/BIP158 support, regtest support,
restricted peer connection support, a separable protocol state machine, and
tests covering header sync, filter sync, reorgs, retries, cache behavior, and
network simulation.

Wasabi stable/current master is not a strict BIP157 P2P adapter target today.
The current standard-filter path obtains filters from a Bitcoin node RPC
interface using `getblockfilter`/related RPC calls, while P2P is used for block
downloads. That does not satisfy the suite requirement that a strict target can
download all needed sync data from the Bitcoin P2P network without RPC,
indexers, or special services.

Wasabi has an open draft P2P compact-filter PR:

- WalletWasabi/WalletWasabi#14546, "Get compact filters from the p2p network"
  (`lontivero/WalletWasabi:bip157`), described by the PR itself as a highly
  buggy, ultra-drafty BIP157 implementation.

Plan:

- Add `adapters/nakamoto` as a normal strict BIP157 adapter.
- Add `adapters/wasabi-p2p-experimental` only against the PR branch if it builds
  and can be driven headlessly. Keep it marked experimental until it lands or
  becomes stable enough to pin reproducibly.
- Do not score current Wasabi master green/orange/red as a strict BIP157 P2P
  client. If useful, add a separate non-strict `wasabi-rpc` comparison adapter
  later, labeled `BIP158-RPC` and excluded from the strict conformance color.

Relevant upstream references to keep in reports:

- Nakamoto repository: https://github.com/cloudhead/nakamoto
- Nakamoto README states client-side block filtering is implemented and working.
- Nakamoto issue #155: filters and blocks can go out of sync on unstable
  networks.
- Nakamoto issue #156: cached filters cannot be rescanned without retrieval in
  some usage patterns.
- Nakamoto issue #102: fast rescan plus low gap limit can miss blocks.
- Nakamoto PRs #53, #57, #64, #72, #73, #76, #77, #81, #96, and #97 cover
  compact-filter cache, range, timeout, rollback, retry, and race fixes.
- Wasabi repository: https://github.com/WalletWasabi/WalletWasabi
- Wasabi PR #13794 and PR #14345 moved standard-filter sync to Bitcoin RPC.
- Wasabi PR #14546 is the open P2P compact-filter branch.
- Wasabi issues #14468, #14455, #14201, #14094, #3668, #4190, #9804, #10219,
  and #13108 provide sync, recovery, block-download, and privacy scenario ideas.

## Why Some Scenarios Are Skipped

The catalog is not an execution manifest. It is coverage accounting.

`harness.Run` starts by marking every catalog entry as `skipped`, then
overwrites only the scenarios that have executable harness code and enough
prerequisite progress to assert a result. This makes missing coverage visible
instead of silently omitting it.

A cataloged scenario can become executable only after all needed pieces exist:

- chain fixture support for the required block, reorg, script, or filter shape
- peer simulator behavior for the required P2P messages and faults
- harness assertions for the expected result
- adapter API support or a clear way to infer the result from peer transcripts
  and wallet outcomes

## Scenarios Runnable Today

These run without an adapter:

- `bip158.coinbase_input_excluded`
- `bip158.coinbase_output_included`
- `bip158.empty_filter_wire_forms`
- `bip158.full_script_not_pushdata`
- `bip158.op_return_excluded`
- `bip158.prevout_legacy_included`
- `bip158.prevout_p2sh_included`
- `bip158.prevout_p2wsh_included`
- `bip158.prevout_taproot_included`
- `bip158.zero_element_serialization`

These additionally run when `bip157-harness` is given `--adapter-url`:

- `adapter.honest_wallet_receive_spend`
- `kyoto.various_client_methods`
- `kyoto.whitelist_only_sync`
- `chain.long_checkpointed_header_sync`
- `bip157.large_batch_progress_timeout`
- `bip157.cfheaders_order_and_checkpoint_boundaries`
- `bip157.bad_cfcheckpt_response`
- `bip157.bad_cfheaders_prev_header`
- `bip157.conflict_one_honest_one_liar`
- `bip157.direct_bad_cfilter_ban`
- `bip157.wrong_filter_type_response`
- `network.outage_filter_headers`
- `network.outage_block_download`
- `neutrino.sync_without_headers_import.initial_sync`
- `neutrino.sync_without_headers_import.one_shot_rescan`
- `neutrino.sync_without_headers_import.long_rescan_start`
- `neutrino.sync_without_headers_import.rescan_results`
- `neutrino.blockmanager_invalid_interval.invalid_prev_header`
- `neutrino.cfcheckpt_sanity.case_1`
- `neutrino.cfheaders_mismatch.case_1`
- `neutrino.detect_bad_peers.filter_hash_mismatch`
- `neutrino.detect_bad_peers.unresponsive_peer`

Some baseline IDs above are asserted by the same underlying conformance run. If
a prerequisite sync fails before the assertion point, dependent baseline IDs may
remain skipped or fail with the prerequisite obstacle.

Practical command shapes:

```sh
go run ./cmd/bip157-harness --out run-artifacts/no-adapter
```

```sh
go run ./cmd/fake-adapter --listen 127.0.0.1:0
go run ./cmd/bip157-harness \
  --adapter-url http://127.0.0.1:<adapter-port> \
  --data-dir /tmp/bip157-adapter \
  --out run-artifacts/fake
```

## Priority 1: Add Nakamoto and Wasabi Targets

Tasks:

1. Build `adapters/nakamoto` as a Rust adapter that embeds or drives Nakamoto
   in regtest mode with a harness-supplied restricted peer set.
2. Expose the existing adapter API:
   - start watching a script
   - return visible transactions for watched scripts
   - return best known block hash
   - return block hash by height
   - report peer connection state and peer punishment if available
3. Verify that Nakamoto does not use DNS seeds, address gossip, RPC, or any
   uncontrolled network source when the harness gives explicit peers.
4. Add reproducible Nix build wiring for the Nakamoto adapter.
5. Fetch and pin the Wasabi P2P compact-filter PR branch in an experimental
   adapter package.
6. Determine whether the Wasabi PR branch can run headlessly on regtest and
   accept harness-controlled P2P peers without RPC.
7. If the Wasabi PR branch is usable, implement
   `adapters/wasabi-p2p-experimental`; otherwise record the exact blocker and
   keep current Wasabi master classified as non-strict `BIP158-RPC`.
8. Add adapter documentation explaining the difference between:
   - strict BIP157 P2P adapters
   - Wasabi's current RPC standard-filter mode
   - the experimental Wasabi P2P PR branch

Exit criteria:

- Nakamoto runs in the same strict matrix as Kyoto and Neutrino.
- Wasabi is either runnable through the P2P PR branch or explicitly reported as
  blocked/not a current strict BIP157 P2P client.
- No strict adapter needs Bitcoin RPC, a Wasabi indexer, Electrum, esplora, or
  another non-P2P service.

## Priority 2: Upstream-Derived Scenario Expansion

The following scenario ideas came from Nakamoto and Wasabi tests, issues, and
PRs. Add only scenarios that do not contradict BIP157/BIP158; optional privacy
and product-recovery checks must be labeled `IMPLEMENTATION` or `MAY`.

Nakamoto-derived tasks:

1. Add out-of-order `block` and `cfilter` delivery where the final wallet result
   must be invariant.
2. Add variable `cfheaders` granularity: one header per message, maximum-size
   messages, and uneven splits over the same range.
3. Add overlapping in-flight `cfheaders` ranges and require the client to ignore
   stale overlap without corrupting the filter-header chain.
4. Add single-peer and multi-peer `getcfheaders` timeout/retry behavior.
5. Add `getcfilters` retry from the next unprocessed filter height after partial
   progress.
6. Add cache hit tests: full cache hit, partial overlap at min/max boundaries,
   cache gaps, and cache eviction during rescan.
7. Add cached-filter rescan without P2P refetch where an implementation claims
   persistent compact-filter caching.
8. Add fast rescan plus low gap-limit/backtracking coverage for wallet adapters
   that derive more watched scripts after a match.
9. Add persisted-store recovery for duplicate or inconsistent block/filter
   header records.
10. Add new block arrival while `cfheaders` or `cfilters` are in flight.
11. Add transaction reorg/reconfirm and stale-transaction status coverage where
    the adapter exposes submitted transaction state.
12. Add restricted-connect/no-discovery coverage for explicit-peer mode.

Wasabi-derived tasks:

1. For the experimental P2P branch, add empty `cfheaders`, wrong range start,
   missing previous filter header, malformed GCS filter, filter/header mismatch,
   and block-hash/height mismatch cases.
2. Add a large-range lookahead test matching the Wasabi PR's shape: multiple
   concurrent header/filter assignments, out-of-order completion, and stale
   assignment timeout/reassignment.
3. Add a reorg check where the filter-header chain and block-header chain
   disagree within the recent lookback window.
4. Add a block-download adversarial matrix:
   - invalid block is rejected and a different peer is tried
   - slow block peer is treated as timeout, not proven malicious data
   - failed block provider recovers without process restart
5. Add RPC-late-start recovery only for a future non-strict `wasabi-rpc`
   comparison track; do not let it count as BIP157 P2P conformance.
6. Add filter-store corruption and partial-write recovery as an optional storage
   scenario.
7. Add older-wallet/newer-wallet shared-filter progress as an optional
   product-level scenario; this checks that one wallet's earlier birthday does
   not block already-usable wallets longer than needed.
8. Add optional privacy scenarios for decoy/recent-block downloads. These must
   never be scored as BIP157/BIP158 `MUST`, because the BIPs do not require
   decoy block downloads.

Candidate scenario IDs:

- `nakamoto.out_of_order_blocks_and_filters`
- `bip157.cfheaders_variable_granularity`
- `bip157.cfheaders_overlapping_inflight_ranges`
- `bip157.cfheaders_single_peer_timeout_retry`
- `bip157.getcfilters_partial_progress_retry`
- `storage.rescan_cached_filters_without_refetch`
- `storage.filter_cache_overlap_and_gaps`
- `wallet.fast_rescan_gap_limit_backtrack`
- `persistence.duplicate_header_store_recovery`
- `chain.new_block_during_filter_sync`
- `chain.tx_reorg_reconfirm_stale_status`
- `network.restricted_connect_no_discovery`
- `bip157.empty_cfheaders_response`
- `bip157.malformed_gcs_filter_payload`
- `bip157.cfilter_block_hash_sequence_mismatch`
- `bip157.concurrent_range_lookahead_reassignment`
- `chain.filter_header_block_header_recent_mismatch`
- `blocks.invalid_downloaded_block_rejected`
- `network.slow_block_peer_retry`
- `storage.filter_store_corruption_recovery`
- `wallet.shared_filter_progress_by_birth_height`
- `privacy.decoy_recent_block_downloads`

Exit criteria:

- Every scenario imported from Nakamoto or Wasabi has a source link in
  documentation and a BIP classification (`MUST`, `SHOULD`, `MAY`, or
  `IMPLEMENTATION`).
- Scenarios already covered by the current suite are not duplicated; they are
  cross-referenced to the existing scenario ID.
- Optional product scenarios are clearly separated from strict conformance.

## Priority 3: Explain Neutrino Header Pagination Failure

Neutrino currently disconnects after peerlab replies to the first `getheaders`
with 2000 headers. It does not request the next page or compact-filter data.

Tasks:

1. Compare peerlab's header response shape against btcd/bitcoind behavior used
   by Neutrino's existing rpctest/SimNet tests.
2. Add a focused compatibility test for 2000-header pagination.
3. Determine whether the failure is peerlab behavior, adapter configuration, or
   a Neutrino bug.
4. Re-run the full Neutrino matrix after the root cause is fixed.

Exit criteria:

- Neutrino either reaches height 2005 and starts compact-filter checks, or the
  report identifies the exact protocol incompatibility.

## Priority 4: Reorg, Persistence, and Canonicality

Tasks:

1. Build forked fixtures and mutable peer behavior for one-block and two-block
   reorgs.
2. Interrupt `cfheaders` processing, restart with the same data directory, and
   require compact-filter headers to catch up without data deletion.
3. Persist an invalid filter-header chain from mutually consistent malicious
   peers, restart, introduce an honest peer, and require rollback/resync.
4. Run a long reorg while filters are in flight.
5. Add stale-branch block request tests for APIs that expose block fetching by
   hash.

Scenario IDs:

- `kyoto.live_reorg`
- `kyoto.live_reorg_additional_sync`
- `kyoto.stop_reorg_resync`
- `kyoto.stop_reorg_two_resync`
- `kyoto.stop_reorg_start_on_orphan`
- `persistence.interrupted_cfheaders_restart`
- `bip157.invalid_persisted_filter_headers_recover`
- `chain.reorg_filter_in_flight_long`
- `kyoto.stale_header_block_request`

Exit criteria:

- Recovery does not require manual data-dir deletion.
- Canonical versus known-stale block state is visible in the report.

## Priority 5: Expand Adversarial BIP157 Matrix

Tasks:

1. Add disagreement cases at configurable heights:
   - conflicting `cfheaders`
   - conflicting `cfcheckpt`
   - wrong `cfilter`
   - late peer joins with bad compact-filter state
2. Add bad-data versus timing-race false-ban scenarios:
   - provably invalid compact-filter data
   - ordinary peer disconnect during follow-up query
   - late in-flight compact-filter response
   - new block announcement during in-flight filter sync
3. Add peer-specific stop-hash knowledge tests:
   - peer A knows the stop hash
   - peer B does not
   - client must not enter reconnect loops or classify unknown stop hash as
     proven bad compact-filter data
4. Implement self-consistent eclipse reporting:
   - all peers mutually agree on bad filter headers and filters
   - report the trust limitation clearly instead of pretending full BIP158
     verification occurred

Scenario IDs:

- `bip157.disagreement_interrogation_matrix`
- `peer.bad_data_vs_race_false_ban`
- `network.followup_nonresponse_not_bad_data`
- `bip157.stop_hash_known_by_peer`
- `bip157.self_consistent_eclipse`

Exit criteria:

- One honest peer plus one lying peer can be resolved by block-derived evidence
  where the implementation claims that behavior.
- Timing failures do not count as proof of bad compact-filter data.
- Reports state whether peer punishment was observed, unsupported, or inferred.

## Priority 6: BIP158 Exactness Still Missing

Tasks:

1. Add OP_RETURN conflict-resolution coverage where the only peer difference is
   improper inclusion of OP_RETURN output scripts.
2. Add nested-segwit and non-witness prevout coverage.
3. Add block retrieval with and without witness data when an adapter exposes
   that capability.

Scenario IDs:

- `bip158.op_return_conflict_resolution`
- `blocks.witness_prevout_matrix`

Exit criteria:

- Conflict-resolution scenarios catch invalid filters when at least one honest
  compact-filter source is available.

## Priority 7: Network and Optional Capability Stress

Tasks:

1. Add optional netns/veth/tc `netem` mode:
   - delay
   - packet loss
   - duplication
   - reordering
   - full outage and recovery
2. Add idle keepalive and long-wait checks.
3. Add deterministic restricted-peer-set checks:
   - valid and invalid configured peers
   - DNS-style peers
   - onion-style peers where the adapter supports them
   - no uncontrolled discovery when disabled
4. Add optional BIP130 `sendheaders` tip tracking.
5. Add optional `NODE_NETWORK_LIMITED` near-tip peer behavior.
6. Add optional storage/cache/import tests for adapters that expose those
   capabilities.

Scenario IDs:

- `network.idle_keepalive_and_long_wait`
- `config.restricted_peer_set_and_onion`
- `chain.sendheaders_tip_tracking`
- `services.network_limited_near_tip`
- `storage.filter_cache_persistence`
- `import.sideload_headers_then_p2p_divergence`
- `storage.partial_write_recovery`

Exit criteria:

- Optional scenarios are skipped or marked unsupported unless the adapter
  declares the required capability.
- Mandatory temporary network failures recover without restart.

## Priority 8: Reporting and Coverage Completion

Tasks:

1. Add a BIP157/BIP158 requirement coverage table:
   - every `MUST`
   - every relevant `SHOULD`
   - implemented, skipped, unsupported, or out of scope
2. Add JUnit output for CI.
3. Add a basic CI job for non-privileged mode.
4. Add documentation for privileged `netem` mode.
5. Keep adapter documentation current for Kyoto, Neutrino, and generic
   third-party implementations.

Exit criteria:

- A new implementation author can build an adapter, run the suite, and read a
  report that clearly states conformance color, skipped coverage, unsupported
  capabilities, and failure evidence.

## Scoring Rules To Preserve

- BIP157/BIP158 `MUST` failures are red.
- Missing or failing `SHOULD` behavior is orange unless it causes a mandatory
  correctness failure.
- Optional implementation stress scenarios are `IMPLEMENTATION` or `MAY`.
- A temporary network failure is not proof of malicious compact-filter data.
