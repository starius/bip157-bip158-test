# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-23

This is the current execution plan for `/home/user/bip157-bip158-test`.
Completed bootstrap work has been removed from the plan section so the remaining
items describe work that still needs to be done.

## Goal

Build a reproducible regtest conformance suite for BIP157/BIP158 light-client
implementations.

The suite validates an implementation through a small adapter binary. The
adapter exposes a fixed API, connects only to harness-controlled regtest peers,
and reports best block, watched-script matches, and peer state where available.
The harness creates deterministic chains, honest peers, adversarial peers,
temporary network failures, and conformance reports.

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
- A deterministic short regtest fixture in `chainlab/`.
- Independent BIP158 filter helpers with unit tests for coinbase outputs,
  OP_RETURN exclusion, zero-element serialization, and filter-header linkage.
- A Bitcoin P2P simulator in `peerlab/` that can serve headers, blocks,
  `cfheaders`, `cfilters`, `cfcheckpt`, corrupt selected compact-filter data,
  delay selected responses once, and record transcripts.
- A harness in `harness/` and `cmd/bip157-harness`.
- A reference fake adapter in `cmd/fake-adapter`.
- Kyoto and Neutrino adapters in `adapters/kyoto` and `adapters/neutrino`.
- Scenario metadata in `scenario/catalog.go` that includes the existing Kyoto
  and Neutrino baseline tests plus stronger conformance scenarios.

Latest recorded validation:

- Fake adapter: green for the executable subset.
- Kyoto adapter: orange; honest wallet matching and temporary delay scenarios
  passed, adversarial peer-punishment scenarios failed or could not expose peer
  state cleanly.
- Neutrino adapter: red on the current short fixture; it handshook and fetched
  headers but did not reach compact-filter-ready tip before timeout.

Details are in `VALIDATION_REPORT.md`.

## Why Some Scenarios Are Skipped

The catalog is not an execution manifest. It is coverage accounting.

`harness.Run` currently starts by marking every catalog entry as `skipped`, then
overwrites only the scenarios that have executable harness code. This makes
missing coverage visible in every report instead of silently omitting it.

A cataloged scenario can become executable only after all needed pieces exist:

- chain fixture support for the required block, reorg, script, or filter shape
- peer simulator behavior for the required P2P messages and faults
- harness assertions for the expected result
- adapter API support or a clear way to infer the result from peer transcripts
  and wallet outcomes

Scenarios are therefore skipped because they are not implemented as harness
code yet, not because they are disabled by a runtime flag.

## Scenarios Runnable Today

These run without an adapter:

- `bip158.coinbase_output_included`
- `bip158.prevout_legacy_included`
- `bip158.op_return_excluded`
- `bip158.zero_element_serialization`

These additionally run when `bip157-harness` is given `--adapter-url`:

- `adapter.honest_wallet_receive_spend`
- `bip157.conflict_one_honest_one_liar`
- `bip157.direct_bad_cfilter_ban`
- `network.outage_filter_headers`
- `network.outage_block_download`

Everything else in `scenario.Catalog()` is still catalog-only and will be
reported as `skipped`.

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

The Kyoto and Neutrino adapters can also be run, but current reports should be
read as validation of the executable subset only.

## Priority 1: Long-Chain BIP157 Core

This is the next implementation target because it unlocks real BIP157 range and
checkpoint behavior and likely explains the current Neutrino adapter failure on
the short fixture.

Tasks:

1. Add a deterministic long-chain fixture mode in `chainlab`.
   - Cross at least several `cfheaders`/`cfcheckpt` intervals.
   - Keep watched receive/spend transactions at known heights.
   - Include blocks with empty filters, OP_RETURN-only filters, and ordinary
     wallet matches.
   - Unit-test filter headers, checkpoint headers, and expected matches.

2. Extend `peerlab` for long-range compact-filter behavior.
   - Serve long-chain `getheaders`, `getcfheaders`, `getcfilters`, and
     `getcfcheckpt`.
   - Enforce BIP157 maximum ranges.
   - Support wrong stop hash, stale stop hash, wrong previous filter header,
     out-of-order late response, duplicate response, and too-many response
     cases.

3. Implement these executable scenarios:
   - `chain.long_checkpointed_header_sync`
   - `bip157.large_batch_progress_timeout`
   - `bip157.cfheaders_order_and_checkpoint_boundaries`

4. Re-run fake, Kyoto, and Neutrino adapters.
   - Neutrino should be re-evaluated after the long-chain path exists.
   - If Neutrino still fails, the peer transcript should show the exact missing
     simulator behavior or client-side failure.

Exit criteria:

- The fake adapter remains green.
- The harness can drive a long-chain sync through `peerlab`.
- Reports distinguish range/checkpoint protocol failures from adapter API gaps.

## Priority 2: Adversarial BIP157 Correctness

Tasks:

1. Implement a disagreement interrogation matrix:
   - conflicting `cfheaders`
   - conflicting `cfcheckpt`
   - wrong `cfilter`
   - first disagreement at configurable height
   - late peer joins with bad compact-filter state

2. Implement bad-data versus timing-race false-ban scenarios:
   - provably invalid compact-filter data
   - ordinary peer disconnect during follow-up query
   - late in-flight compact-filter response
   - new block announcement during in-flight filter sync

3. Implement peer-specific stop-hash knowledge tests:
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

## Priority 3: BIP158 Exactness

Tasks:

1. Add full-script matching cases.
   - Construct scripts where the same pushed bytes appear in different script
     templates.
   - Assert matching is against the whole scriptPubKey, not arbitrary pushdata.

2. Add OP_RETURN conflict-resolution coverage.
   - Use a peer-disagreement scenario where the only difference is improper
     inclusion of OP_RETURN output scripts.

3. Add empty and near-empty filter wire-form cases.
   - Verify canonical zero-element serialization and header commitment.

4. Add witness and prevout coverage.
   - P2WPKH
   - P2WSH
   - nested segwit
   - P2TR
   - block retrieval with and without witness data when an adapter exposes that
     capability

Scenario IDs:

- `bip158.full_script_not_pushdata`
- `bip158.op_return_conflict_resolution`
- `bip158.empty_filter_wire_forms`
- `blocks.witness_prevout_matrix`
- `bip158.prevout_taproot_included`

Exit criteria:

- The independent filter builder covers all BIP158 mandatory element rules.
- Conflict-resolution scenarios catch invalid filters when at least one honest
  compact-filter source is available.

## Priority 4: Reorg, Persistence, and Canonicality

Tasks:

1. Interrupt `cfheaders` processing, restart with the same data directory, and
   require compact-filter headers to catch up without data deletion.

2. Persist an invalid filter-header chain from mutually consistent malicious
   peers, restart, introduce an honest peer, and require rollback/resync rather
   than deadlock.

3. Run a long reorg while filters are in flight.
   - Do not permanently ban the only honest peer for stale in-flight responses.
   - Roll back stale filter headers and filters.

4. Add stale-branch block request tests for APIs that expose block fetching by
   hash.
   - Known stale hashes must be distinguishable from canonical chain hashes.

Scenario IDs:

- `persistence.interrupted_cfheaders_restart`
- `bip157.invalid_persisted_filter_headers_recover`
- `chain.reorg_filter_in_flight_long`
- `kyoto.stale_header_block_request`

Exit criteria:

- Recovery does not require manual data-dir deletion.
- Canonical versus known-stale block state is visible in the report.

## Priority 5: Network and Optional Capability Stress

Tasks:

1. Add optional netns/veth/tc `netem` mode.
   - delay
   - packet loss
   - duplication
   - reordering
   - full outage and recovery

2. Add idle keepalive and long-wait checks.

3. Add deterministic restricted-peer-set checks.
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

1. Add a BIP157/BIP158 requirement coverage table.
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
- A self-consistent malicious eclipse is a trust limitation unless an adapter
  claims stronger external validation.
- Skipped scenarios are visible and do not improve the score.
