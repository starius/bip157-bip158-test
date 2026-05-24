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
- Peerlab gives IPv4 peers distinct loopback IP identities with
  `127.27.0.N`. IPv6 currently binds to `::1`, so IPv6 socket behavior works
  but distinct IPv6 peer identity is still unsupported without an address lab.
- The adapter API, harness, and matrix all understand an explicit environment
  dimension: `ipv4`, `ipv6`, `tor-v3`, `i2p`, and `cjdns`.
- Overlay lab manifests exist for Tor v3, I2P, and cjdns. Tor has pinned
  Chutney source available through Nix, but runtime onion-service wiring is
  not active yet.
- Nix-pinned Go, Rust/Cargo, Bitcoin Core, protobuf, .NET 10, Tor, Chutney,
  Java I2P, i2pd, cjdns, iproute2, and helper tooling.
- A tracked latest matrix in `IMPLEMENTATION_MATRIX.md`.
- Latest validation and failure classification in `VALIDATION_REPORT.md`.

Latest validation summary:

- Fake adapter over IPv4: green, 72 pass and 24 skipped.
- Fake adapter over IPv6: green, 71 pass, 1 unsupported, and 24 skipped.
- Kyoto over IPv4: red, 26 pass, 46 fail, and 24 skipped.
- Wasabi over IPv4: red, 42 pass, 30 fail, and 24 skipped.
- Neutrino over IPv4: red, 14 pass, 53 fail, and 29 skipped.
- Nakamoto over IPv4: red, 13 pass, 54 fail, and 29 skipped.
- Real-adapter IPv6, Tor v3, I2P, and cjdns rows are capability or lab
  unsupported rows, not full conformance passes.

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
6. Use the environment identity support from Workstream 6 in empirical bad-peer
   rows. IPv4 already has distinct peer IPs. IPv6 and Tor identity assertions
   must stay unsupported until the address lab and Chutney lab provide distinct
   identities.
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

## Workstream 6: Full IPv6 and Tor v3 Runtime

Goal: turn the existing environment dimension into real conformance runs for
IPv6 and Tor v3. IPv4 is already active. I2P and cjdns remain later overlay
workstreams because they need separate router and namespace runners.

### IPv6 Address Lab

Current blocker: peerlab can bind IPv6 sockets on `::1`, but all simulated
peers share the same IPv6 identity. This is enough to prove basic IPv6
connectivity, but not enough for bad-peer or effective-ban scenarios.

Tasks:

1. Add an `addresslab` package with an explicit allocator interface:
   `Allocate(env, peerIndex)`, `Release()`, and `Capabilities()`.
2. Keep a non-privileged fallback allocator that returns `::1` and marks
   `DistinctPeerIdentities=false`. This keeps ordinary developer runs useful
   and honest.
3. Add a Linux `iproute2` allocator for privileged validation runs:
   - create a deterministic ULA prefix such as `fd7a:b157:b158::/64`;
   - add one `/128` address per peer to `lo` or a dedicated dummy interface;
   - bind peerlab listeners to those addresses;
   - remove every address during cleanup, even after failed scenarios.
4. Teach `peerlab.StartInEnvironment` to request addresses from the allocator
   instead of hard-coding `::1` for all IPv6 peers.
5. Add harness flags for address-lab selection and strict identity behavior:
   - `--address-lab=auto|loopback|linux-iproute`;
   - `--require-distinct-identities`, used by bad-peer rows that cannot be
     interpreted safely when identities collapse.
6. Update adapter configuration tests so bracketed IPv6 peer addresses,
   identities, and transcript evidence are preserved end to end.
7. Enable real adapters for IPv6 only after each adapter proves it can connect
   to bracketed IPv6 peers in connect-only regtest mode.
8. Run the full matrix over IPv6. A passing IPv6 row must mean the same
   scenarios ran over IPv6, not that the adapter reported the environment as
   unsupported.

Validation:

- Unit tests for the allocator interface, fallback mode, Linux command
  planning, cleanup ordering, and peer identity metadata.
- A fake-adapter IPv6 run in both fallback and distinct-identity modes.
- Real-adapter IPv6 runs for every adapter that claims support.
- Empirical bad-peer rows over IPv6 once distinct identities are available.

### Tor v3 Chutney Lab

Current blocker: Chutney is pinned and manifests can be generated, but the
harness does not yet start a private Tor network, create onion services, or
route adapters through SOCKS.

Tasks:

1. Replace the current manifest-only `tor-v3` path with a `torlab` runtime
   package that owns one isolated Chutney data directory per harness run.
2. Start a private Chutney network with local directory authorities, relays,
   one SOCKS client endpoint, and a control endpoint suitable for creating
   onion services.
3. Add a small Tor control client or wrapper that can:
   - authenticate to the Chutney-controlled Tor instance;
   - create one ephemeral v3 onion service per peerlab listener with
     `ADD_ONION`;
   - map onion service ports to local `127.0.0.1:<peerlab-port>`;
   - read the resulting `.onion` names and expose them as peer identities;
   - tear the services down during scenario cleanup.
4. Add a harness lab lifecycle:
   - start peerlab on local clear TCP;
   - expose each listener through a distinct onion service;
   - set `PeerConfig.Address` to `<service>.onion:<port>`;
   - set `PeerConfig.Transport=tor-v3`;
   - set `PeerConfig.ProxyAddress` and `Environment.ProxyAddress` to the
     Chutney SOCKS endpoint;
   - wait until each onion service is reachable before starting the adapter.
5. Remove the blanket `!env.IsClearTCP()` skip for `tor-v3` when an active
   `torlab` is available. Keep the unsupported result only when the lab cannot
   start or the adapter does not claim Tor support.
6. Teach adapters to use the supplied SOCKS endpoint or native onion transport:
   - fake adapter first, as the harness proof;
   - Wasabi next, because its application stack already has Tor concepts;
   - Neutrino, Kyoto, and Nakamoto through small dialer or connector patches
     where their libraries do not already expose proxy dialing.
7. Add Tor-specific temporary-network scenarios:
   - stop and restart one onion service while the Tor network stays up;
   - stop and restart the Chutney client SOCKS endpoint;
   - delay onion reachability before adapter start;
   - verify recovery without accepting bad filter data or needing restart.
8. Run the full BIP157/BIP158 matrix over Tor v3 for every adapter that claims
   support. Failures after a support claim score normally; missing onion
   support remains an environment capability result.

Validation:

- Unit tests for Tor control command encoding, onion endpoint parsing, and lab
  cleanup ordering.
- A short integration smoke test that starts Chutney, creates one onion service
  for a local TCP echo server, reaches it through SOCKS, and shuts down cleanly.
- A fake-adapter full Tor run before enabling real adapters.
- Real-adapter Tor runs with the same scenario IDs as IPv4 and IPv6.

Scenario IDs:

- `env.ipv6.full_matrix`
- `env.tor_v3.full_matrix`
- `peer.identity_distinct_ipv6`
- `peer.identity_distinct_overlay`
- `network.tor_onion_service_restart`
- `network.tor_socks_endpoint_restart`
- `network.tor_delayed_onion_reachability`

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
