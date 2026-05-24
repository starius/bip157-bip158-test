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
- Scenario metadata in `scenario/catalog.go` that includes the existing Kyoto
  and Neutrino baseline tests plus stronger conformance scenarios.

Latest recorded validation is in `VALIDATION_REPORT.md`:

- Fake adapter: green.
- Kyoto adapter: red because the malformed `cfheaders` previous-header scenario
  is a mandatory failure; several SHOULD adversarial peer checks also fail.
- Neutrino adapter: red because it disconnects after peerlab's first 2000-header
  page and does not reach compact-filter sync.

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

## Priority 1: Explain Neutrino Header Pagination Failure

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

## Priority 2: Reorg, Persistence, and Canonicality

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

## Priority 3: Expand Adversarial BIP157 Matrix

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

## Priority 4: BIP158 Exactness Still Missing

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

## Priority 5: Network and Optional Capability Stress

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

## Priority 6: Reporting and Coverage Completion

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
