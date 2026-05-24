# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-24

This is the active plan after Wasabi became a strict adapter target and the
expanded adversarial rows were run. Completed Nix pinning, adapter scaffolding,
matrix generation, and already-revived scenarios are intentionally omitted
from the active task list.

## Current State

The suite has:

- A fixed adapter schema in `proto/bip157test.proto` and HTTP/JSON API types in
  `api/`.
- Deterministic short and long regtest fixtures in `chainlab/`; the long
  fixture reaches height 2005 and crosses two compact-filter checkpoint
  intervals.
- A peer simulator in `peerlab/` that serves headers, blocks, `cfheaders`,
  `cfilters`, and `cfcheckpt`, and can inject bad data and temporary delays.
- A harness in `harness/`, CLI runners in `cmd/`, and a cross-implementation
  matrix generator.
- Fake, Kyoto, Neutrino, Nakamoto, and Wasabi adapters.
- Nix-pinned Go, Rust/Cargo, Bitcoin Core, protobuf, and .NET 10 tooling.
- A tracked latest matrix in `IMPLEMENTATION_MATRIX.md`.
- Latest validation and failure classification in `VALIDATION_REPORT.md`.

Latest validation summary:

- Fake adapter: green, 70 pass and 18 skipped.
- Kyoto adapter: red, 24 pass, 46 fail, and 18 skipped.
- Wasabi adapter: red, 24 pass, 46 fail, and 18 skipped.
- Neutrino adapter: red, 12 pass, 53 fail, and 23 skipped.
- Nakamoto adapter: red, 11 pass, 54 fail, and 23 skipped.

## Active Coverage

The active suite covers:

- BIP158 element rules for coinbase output inclusion, coinbase input
  exclusion, OP_RETURN exclusion, full script matching, empty filters,
  zero-element serialization, legacy prevouts, P2SH, P2WSH, and Taproot.
- Basic adapter behavior: configure, start, stop, watch script, wallet
  receive/spend matches, best block, block hash lookup, and peer listing.
- BIP157 long-chain sync across compact-filter checkpoint boundaries.
- Large-batch progress and timeout behavior.
- Temporary outage recovery for `cfheaders` and block download.
- Explicit harness peer mode with discovery disabled.
- Bad `cfcheckpt`, bad `PrevFilterHeader`, empty `cfheaders`, conflicting
  `cfheaders`, corrupt `cfilter`, malformed GCS payload, wrong `cfilter` block
  hash, wrong filter type, unresponsive peer, scrambled headers, and invalid
  downloaded block behavior.
- Black-box variants of the current Kyoto and Neutrino test scenarios that can
  be represented without internal implementation hooks.

## Workstream 1: Classify and Fix Current Failures

Goal: classify every failed row as a bad scenario, adapter bug, or library bug,
then fix the simple adapter and library cases.

Tasks:

1. Improve adapter observability where the harness can prove that bad data was
   served but cannot prove what the implementation did with that peer.
2. Preserve rows that pass in at least one real implementation; do not weaken
   those scenarios to hide failures elsewhere.
3. Patch adapter-owned bugs directly in this repo.
4. Carry library fixes as local patches with commit-message-quality explainers
   suitable for upstream PRs.
5. Record cases that remain blocked after reasonable debugging in
   `VALIDATION_REPORT.md` and `BIP157_BIP158_FINDINGS.md`.

## Workstream 2: Revive Remaining Skipped Rows

Still skipped in the fake-adapter run:

- `bip157.self_consistent_eclipse`
- `kyoto.live_reorg`
- `kyoto.live_reorg_additional_sync`
- `kyoto.stop_reorg_resync`
- `kyoto.stop_reorg_start_on_orphan`
- `kyoto.stop_reorg_two_resync`
- `kyoto.tx_can_broadcast`
- `neutrino.blockmanager_initial_interval.*`
- `neutrino.import_then_p2p_sync`
- `neutrino.sync_with_headers_import`
- `neutrino.sync_without_headers_import.random_blocks_filters`

Tasks:

1. Build forked fixtures and mutable peer behavior for one-block and two-block
   reorgs.
2. Add restart support in the harness so interrupted `cfheaders`, persisted
   bad filter headers, and import-then-P2P divergence can be tested.
3. Add a transaction-broadcast endpoint to the adapter API before activating
   `kyoto.tx_can_broadcast`.
4. Translate Neutrino's initial interval permutations into black-box peerlab
   behavior without relying on internal block manager hooks.
5. Add randomized deterministic block/filter generation for the
   `random_blocks_filters` row.
6. Implement self-consistent eclipse reporting as a trust-limit scenario, not
   as a pass/fail proof of bad data.

## Workstream 3: Expand Adversarial BIP157 Coverage

Tasks:

1. Add disagreement cases at configurable heights for `cfheaders`,
   `cfcheckpt`, and `cfilter`.
2. Add bad-data versus timing-race false-ban scenarios.
3. Add peer-specific stop-hash knowledge tests.
4. Add partial `getcfilters` progress and retry coverage.
5. Add concurrent range/lookahead reassignment coverage from the Wasabi P2P PR
   design.

Scenario IDs:

- `bip157.disagreement_interrogation_matrix`
- `peer.bad_data_vs_race_false_ban`
- `network.followup_nonresponse_not_bad_data`
- `bip157.stop_hash_known_by_peer`
- `bip157.getcfilters_partial_progress_retry`
- `bip157.concurrent_range_lookahead_reassignment`

## Workstream 4: BIP158 Exactness

Tasks:

1. Add OP_RETURN conflict-resolution coverage where the only peer difference is
   improper inclusion of OP_RETURN output scripts.
2. Add nested-segwit and non-witness prevout coverage beyond the current vector
   rows.
3. Add block retrieval with and without witness data when an adapter exposes
   that capability.

Scenario IDs:

- `bip158.op_return_conflict_resolution`
- `blocks.witness_prevout_matrix`

## Workstream 5: Network and Optional Capability Stress

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

## Workstream 6: Reporting and CI

Tasks:

1. Add a BIP157/BIP158 requirement coverage table.
2. Add JUnit output for CI.
3. Add a basic CI job for non-privileged mode.
4. Add documentation for privileged `netem` mode.
5. Keep adapter documentation current for Kyoto, Neutrino, Nakamoto, Wasabi,
   and third-party implementations.

## Scoring Rules

- BIP157/BIP158 `MUST` failures are red.
- Missing or failing `SHOULD` behavior is orange unless it causes a mandatory
  correctness failure.
- Optional implementation stress scenarios are `OTHER` unless the BIPs impose
  a stronger requirement.
- A temporary network failure is not proof of malicious compact-filter data.
