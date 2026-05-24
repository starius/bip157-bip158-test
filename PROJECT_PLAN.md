# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-23

This is the current active plan for the conformance suite. Completed adapter,
matrix, Nix, and first adversarial-scenario work has been removed from the
active task list.

## Current Built State

The suite now has:

- A fixed adapter schema in `proto/bip157test.proto` and HTTP/JSON API types in
  `api/`.
- Deterministic short and long regtest fixtures in `chainlab/`; the long
  fixture reaches height 2005 and crosses two compact-filter checkpoint
  intervals.
- A peer simulator in `peerlab/` that serves headers, blocks, `cfheaders`,
  `cfilters`, and `cfcheckpt`, and can inject corrupt filter data, corrupt
  `PrevFilterHeader`, empty `cfheaders`, wrong filter types, wrong `cfilter`
  block hashes, invalid downloaded blocks, and temporary delays.
- A harness in `harness/`, CLI runners in `cmd/`, and a cross-implementation
  matrix generator.
- Fake, Kyoto, Neutrino, and Nakamoto adapters.
- Nix-pinned Go, Rust/Cargo, Bitcoin Core, protobuf, and .NET 10 tooling.
- A tracked latest matrix in `IMPLEMENTATION_MATRIX.md`.
- Latest validation in `VALIDATION_REPORT.md`.

Current validation summary:

- Fake adapter: green.
- Kyoto adapter: red; ordinary sync and recovery pass, adversarial validation
  and invalid-block checks fail.
- Neutrino adapter: red; disconnects after peerlab's first 2000-header page.
- Nakamoto adapter: red; connects to peerlab but does not reach the fixture tip.
- Wasabi: not scored as a strict target. Master uses RPC filters; the P2P PR is
  experimental and does not yet expose harness-controlled regtest peers through
  the normal headless startup path.

## Runnable Scenarios

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

These additionally run with `--adapter-url`:

- `adapter.honest_wallet_receive_spend`
- `kyoto.various_client_methods`
- `kyoto.whitelist_only_sync`
- `chain.long_checkpointed_header_sync`
- `bip157.large_batch_progress_timeout`
- `bip157.cfheaders_order_and_checkpoint_boundaries`
- `bip157.bad_cfcheckpt_response`
- `bip157.bad_cfheaders_prev_header`
- `bip157.empty_cfheaders_response`
- `bip157.conflict_one_honest_one_liar`
- `bip157.direct_bad_cfilter_ban`
- `bip157.malformed_gcs_filter_payload`
- `bip157.cfilter_block_hash_sequence_mismatch`
- `bip157.wrong_filter_type_response`
- `blocks.invalid_downloaded_block_rejected`
- `network.outage_filter_headers`
- `network.outage_block_download`
- `network.restricted_connect_no_discovery`
- `neutrino.sync_without_headers_import.initial_sync`
- `neutrino.sync_without_headers_import.one_shot_rescan`
- `neutrino.sync_without_headers_import.long_rescan_start`
- `neutrino.sync_without_headers_import.rescan_results`
- `neutrino.blockmanager_invalid_interval.invalid_prev_header`
- `neutrino.cfcheckpt_sanity.case_1`
- `neutrino.cfheaders_mismatch.case_1`
- `neutrino.detect_bad_peers.filter_hash_mismatch`
- `neutrino.detect_bad_peers.unresponsive_peer`

## Active Priority 1: Isolate Adapter/Peerlab Compatibility

Neutrino and Nakamoto both fail before useful compact-filter adversarial
evidence is available. These are now the highest-leverage blockers.

Tasks:

1. Compare peerlab's 2000-header response shape with btcd/bitcoind behavior
   used by Neutrino's existing rpctest/SimNet tests.
2. Add a focused compatibility test for header pagination and stop-hash
   handling.
3. Determine why Nakamoto asks peerlab for the genesis filter and does not
   advance to the long-chain tip.
4. Add focused peerlab transcript assertions for Nakamoto's expected startup
   sequence.
5. Re-run Neutrino and Nakamoto after root cause fixes.

Exit criteria:

- Each adapter either reaches height 2005 and starts compact-filter checks, or
  the report identifies the exact protocol incompatibility.

## Active Priority 2: Reorg, Persistence, and Canonicality

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

## Active Priority 3: Expand Adversarial BIP157 Matrix

Tasks:

1. Add disagreement cases at configurable heights for `cfheaders`,
   `cfcheckpt`, and `cfilter`.
2. Add bad-data versus timing-race false-ban scenarios.
3. Add peer-specific stop-hash knowledge tests.
4. Implement self-consistent eclipse reporting.
5. Add partial `getcfilters` progress and retry coverage.
6. Add concurrent range/lookahead reassignment coverage from the Wasabi P2P PR
   design.

Scenario IDs:

- `bip157.disagreement_interrogation_matrix`
- `peer.bad_data_vs_race_false_ban`
- `network.followup_nonresponse_not_bad_data`
- `bip157.stop_hash_known_by_peer`
- `bip157.self_consistent_eclipse`
- `bip157.getcfilters_partial_progress_retry`
- `bip157.concurrent_range_lookahead_reassignment`

## Active Priority 4: BIP158 Exactness Still Missing

Tasks:

1. Add OP_RETURN conflict-resolution coverage where the only peer difference is
   improper inclusion of OP_RETURN output scripts.
2. Add nested-segwit and non-witness prevout coverage.
3. Add block retrieval with and without witness data when an adapter exposes
   that capability.

Scenario IDs:

- `bip158.op_return_conflict_resolution`
- `blocks.witness_prevout_matrix`

## Active Priority 5: Network and Optional Capability Stress

Tasks:

1. Add optional netns/veth/tc `netem` mode for delay, packet loss,
   duplication, reordering, and full outage/recovery.
2. Add idle keepalive and long-wait checks.
3. Add restricted-peer-set variants for valid, invalid, DNS-style, and
   onion-style peers.
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

## Active Priority 6: Reporting and CI

Tasks:

1. Add a BIP157/BIP158 requirement coverage table.
2. Add JUnit output for CI.
3. Add a basic CI job for non-privileged mode.
4. Add documentation for privileged `netem` mode.
5. Keep adapter documentation current for Kyoto, Neutrino, Nakamoto, Wasabi,
   and third-party implementations.

## Scoring Rules To Preserve

- BIP157/BIP158 `MUST` failures are red.
- Missing or failing `SHOULD` behavior is orange unless it causes a mandatory
  correctness failure.
- Optional implementation stress scenarios are `OTHER` unless the BIPs impose
  a stronger requirement.
- A temporary network failure is not proof of malicious compact-filter data.
