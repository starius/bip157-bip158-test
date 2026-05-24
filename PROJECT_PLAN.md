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
- Current peerlab peers bind to `127.0.0.1:0`, so they have separate ports but
  not separate IP identities. This is sufficient for most P2P behavior but not
  sufficient for IP-level ban assertions.
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
5. Add empirical bad-peer checks so the suite is not limited to adapter
   reported `banned` fields:
   - `peer.bad_data_rejected`: after a provably bad `cfheaders`, `cfcheckpt`,
     `cfilter`, or block response, the client must not accept or persist the
     bad data and must recover through valid data.
   - `peer.effective_ban_reconnect`: after a peer serves provably bad data,
     keep the same peer available and trigger a follow-up request. The client
     fails this row if it reconnects to that peer and requests or accepts BIP157
     data from it again within the test window.
6. Add peerlab support for distinct peer identities at the IP level. The
   current suite binds every peer to `127.0.0.1:0`, so peers have distinct
   ports but the same IP. Effective-ban scenarios need separate loopback IPs,
   network namespaces, or an explicit fallback mode that marks IP-level ban
   assertions unsupported when only one loopback address is available.
7. Record cases that remain blocked after reasonable debugging in
   `VALIDATION_REPORT.md` and `BIP157_BIP158_FINDINGS.md`.

Backend-specific next probes:

- Kyoto:
  - Keep `/list-peers` available even when Kyoto's `peer_info()` call fails.
    Return configured peers, last successful peer state, and the latest
    requester or event-loop error instead of HTTP 503.
  - Add adapter-side tracking for disconnect time, last validation error,
    last bad P2P command observed by peerlab, and whether the peer was retried.
  - Patch Kyoto if needed to expose per-peer disconnect reason, bad-peer or
    ban state, and compact-filter validation errors through a public debug API.
  - Inspect Kyoto's stored header graph and compact-filter header chain after
    bad `cfheaders` and bad `cfilter` rows. Record whether the bad data was
    persisted, ignored, or left pending.
  - After observability is fixed, patch bad `cfcheckpt`, bad `cfheaders`, bad
    `cfilter`, wrong filter type, and invalid downloaded block handling.
- Wasabi:
  - Treat the adapter as sufficiently observable for normal sync. The remaining
    work is mostly in the patched P2P compact-filter code.
  - Patch the Wasabi P2P code to surface validation failures for bad
    `cfcheckpt`, bad `PrevFilterHeader`, malformed GCS payloads, wrong filter
    types, wrong block hashes, unresponsive peers, scrambled headers, and
    invalid downloaded blocks.
  - Record whether each failure causes retry, disconnect, ban, rejection, or
    silent acceptance. Expose that classification through `/list-peers`.
  - Keep every Wasabi library change as a patch under `nix/patches/wasabi`
    with an upstream-quality explainer.
- Neutrino:
  - Add a debug endpoint or report attachment that reads Neutrino's stored
    header tip, filter-header tip, ban state, and peer state after each failed
    scenario.
  - If public APIs are insufficient, read Neutrino's on-disk stores directly
    from the configured adapter data directory or add a small local library
    patch to expose the needed sync state.
  - Capture the exact disconnect reason after peerlab sends the first
    2000-header page. Distinguish adapter misconfiguration, peerlab protocol
    incompatibility, and Neutrino header-validation failure.
  - Compare peerlab's header-pagination transcript with the behavior of the
    nodes used by Neutrino's SimNet/rpctest tests.
  - Re-test relevant open PRs only after the root cause is known, so the report
    can say whether a PR fixes the specific failure rather than a broad class
    of sync issues.
- Nakamoto:
  - Add a debug endpoint or storage probe for Nakamoto's chain tip,
    compact-filter header tip, peer-manager state, outstanding request state,
    and compact-filter manager errors.
  - If the public handle remains too coarse, inspect Nakamoto's store directly
    or patch the library to expose the needed chain and compact-filter state.
  - Explain why the adapter often requests `getcfilters` for regtest genesis
    and does not advance to the height-2005 header chain.
  - Verify that the adapter is putting Nakamoto into strict regtest,
    connect-only mode with the harness-supplied peers and no discovery.
  - Once the startup blocker is fixed, rerun the adversarial rows before
    classifying any compact-filter behavior as a library bug.

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
- `peer.bad_data_rejected`
- `peer.effective_ban_reconnect`
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

## Workstream 6: Addressing and Overlay Matrix

Goal: run the full conformance matrix across the address and transport
environments that BIP157/BIP158 clients are expected to use in practice. The
same scenario IDs should be reused; the matrix gains an environment dimension
in addition to implementation and BIP status.

Required environments:

- IPv4 clear TCP.
- IPv6 clear TCP.
- Tor v3 onion service.
- I2P destination.
- cjdns IPv6 overlay address.

Research notes:

- Tor: use Chutney. The Tor Project describes Chutney as a Tor test network
  configuration tool that can launch Tor processes and establish connectivity
  between them. This is the clearest local private-network option.
- I2P: no direct Chutney equivalent was identified. I2P exposes SAM for local
  applications. The I2P SAM docs explicitly recommend testing with both Java
  I2P and i2pd because they are independent router implementations with
  different defaults. A private lab likely needs a harness-owned wrapper around
  multiple routers, local reseed data, floodfill/bootstrap configuration, and
  SAM stream tunnels.
- cjdns: no direct Chutney equivalent was identified. cjdns is an encrypted
  IPv6 network where `cjdroute` is one node, and node configs can specify
  peers in `connectTo`. A private lab likely needs a harness-owned wrapper that
  starts multiple `cjdroute` instances in network namespaces or containers,
  wires their configs together, and exposes the generated cjdns IPv6 addresses
  to adapters.

Reference anchors:

- Tor research tools and Chutney:
  `https://research.torproject.org/tools/`,
  `https://gitlab.torproject.org/tpo/core/chutney`.
- I2P SAM and i2pd tunnel configuration:
  `https://i2p.net/en/docs/api/samv3/`,
  `https://docs.i2pd.website/en/latest/user-guide/tunnels/`.
- cjdns overview and configuration:
  `https://github.com/cjdelisle/cjdns`,
  `https://raw.githubusercontent.com/cjdelisle/cjdns/master/doc/configure.md`.

Tasks:

1. Extend peerlab to bind peers to distinct local identities:
   - IPv4: allocate separate loopback addresses, for example `127.0.0.2`,
     `127.0.0.3`, or netns-local addresses.
   - IPv6: allocate separate loopback or netns-local IPv6 addresses.
   - Record the selected IP family and identity in every peer transcript.
2. Extend the adapter API so the harness can describe peer address type and
   proxy requirements without overloading the plain `address` string.
3. Add an environment selector to the harness and matrix generator:
   `ipv4`, `ipv6`, `tor-v3`, `i2p`, and `cjdns`.
4. Add IPv4 and IPv6 clear-TCP runs first. These should be mandatory for the
   core matrix because they require no anonymity-overlay bootstrap.
5. Add Tor v3 runs through Chutney:
   - start a private Chutney network in the test data directory;
   - expose peerlab nodes as v3 onion services;
   - configure adapters through SOCKS or native onion peer support;
   - run every scenario that can work over Tor;
   - classify unsupported onion peer support separately from BIP157 failures.
6. Add I2P runs through a new `i2p-lab` helper:
   - start multiple local Java I2P or i2pd routers with isolated data
     directories;
   - create local streaming destinations for peerlab;
   - provide deterministic bootstrap using local reseed data or a controlled
     floodfill/router-info seed;
   - connect adapters through SAM or native I2P peer support;
   - test both Java I2P and i2pd where practical because their defaults differ.
7. Add cjdns runs through a new `cjdns-lab` helper:
   - generate per-node cjdroute configs;
   - run nodes in network namespaces or containers;
   - connect nodes through explicit `connectTo` entries;
   - expose peerlab over cjdns IPv6 addresses;
   - collect cjdns peer state as part of scenario evidence.
8. Add a capability model so an implementation can be marked unsupported for an
   environment without turning every row red. If an implementation claims
   support for an environment, failures in that environment should score
   normally.
9. For empirical ban tests, require distinct peer identities in the active
   environment. If the environment collapses all peers to one local IP or one
   proxy endpoint, mark IP-level ban assertions unsupported and fall back to
   behavior-level rejection tests.
10. Add Nix packages or pinned source builds for Chutney, Tor, Java I2P/i2pd,
    cjdns, and any helper tooling needed to create namespaces or containers.

Scenario IDs:

- `env.ipv4.full_matrix`
- `env.ipv6.full_matrix`
- `env.tor_v3.full_matrix`
- `env.i2p.full_matrix`
- `env.cjdns.full_matrix`
- `peer.identity_distinct_ipv4`
- `peer.identity_distinct_ipv6`
- `peer.identity_distinct_overlay`

## Workstream 7: Reporting and CI

Tasks:

1. Add a BIP157/BIP158 requirement coverage table.
2. Add JUnit output for CI.
3. Add a basic CI job for non-privileged mode.
4. Add documentation for privileged `netem` mode.
5. Keep adapter documentation current for Kyoto, Neutrino, Nakamoto, Wasabi,
   and third-party implementations.
6. Add environment-aware matrix output with rows keyed by scenario and columns
   split by implementation plus environment.

## Scoring Rules

- BIP157/BIP158 `MUST` failures are red.
- Missing or failing `SHOULD` behavior is orange unless it causes a mandatory
  correctness failure.
- Optional implementation stress scenarios are `OTHER` unless the BIPs impose
  a stronger requirement.
- A temporary network failure is not proof of malicious compact-filter data.
