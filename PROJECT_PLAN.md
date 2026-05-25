# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-25

This is the active plan after Wasabi became a strict adapter target and the
expanded adversarial rows were run. Completed Nix pinning, adapter scaffolding,
matrix generation, revived scenarios, IPv6 address-lab support, and the
Chutney-backed Tor v3 harness are intentionally omitted from the active task
list.

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
  `127.27.0.N`.
- The `addresslab` package supports non-privileged IPv6 loopback runs and a
  Linux `iproute2` lab that assigns deterministic `fd7a:b157:b158::/64`
  addresses for distinct IPv6 peer identities.
- The adapter API, harness, and matrix all understand an explicit environment
  dimension: `ipv4`, `ipv6`, `tor-v3`, `i2p`, and `cjdns`.
- The `torlab` package starts a private Chutney Tor network, creates ephemeral
  v3 onion services for peerlab listeners, and passes the Chutney SOCKS
  endpoint to adapters.
- Overlay lab manifests exist for I2P and cjdns, but those runtimes are not
  active yet.
- Nix-pinned Go, Rust/Cargo, Bitcoin Core, protobuf, .NET 10, Tor, Chutney,
  Java I2P, i2pd, cjdns, iproute2, and helper tooling.
- A tracked latest matrix in `IMPLEMENTATION_MATRIX.md`.
- Latest validation and failure classification in `VALIDATION_REPORT.md`.

Latest validation summary:

- Fake adapter over IPv4: green, 72 pass and 24 skipped.
- Fake adapter over IPv6: green, 72 pass and 24 skipped.
- Fake adapter over Tor v3: green, 72 pass and 24 skipped.
- Kyoto over IPv4: red, 26 pass, 46 fail, and 24 skipped.
- Wasabi over IPv4: red, 31 pass, 41 fail, and 24 skipped.
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
6. Use the IPv4, IPv6, and Tor identity support from Workstream 6 in empirical
   bad-peer rows. I2P and cjdns identity assertions must stay unsupported until
   those labs provide active overlay identities.
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

Goal: keep the existing environment dimension honest across IPv4, IPv6, Tor
v3, I2P, and cjdns. IPv4, IPv6, and Tor v3 have active harness runtimes. I2P
and cjdns remain later overlay workstreams because they need separate router
and namespace runners.

### IPv6 Address Lab

Completed:

- `addresslab` has loopback and Linux `iproute2` allocators.
- `--address-lab` selects `auto`, `loopback`, or `linux-iproute`.
- `--require-distinct-identities` makes collapsed peer identity a harness
  failure for rows that need separable peers.
- The Linux allocator keeps IPv6 `/128` leases until harness shutdown so
  peerlab listeners cannot lose their assigned addresses between scenarios.
- The fake adapter passed the full IPv6 matrix with distinct ULA peer
  identities.

Remaining:

1. Add an adapter-level IPv6 smoke scenario that starts one honest peer on a
   Linux ULA address, configures the adapter with a bracketed
   `[fd7a:b157:b158::N]:port` peer address, disables discovery, and requires
   the adapter to complete a version/verack handshake. This isolates transport
   support from the broader BIP157/BIP158 correctness failures.
2. Add adapter unit tests that deserialize and preserve these fields from
   `/configure`:
   - `environment.id=ipv6`;
   - `environment.address_type=ipv6`;
   - `environment.transport=tcp`;
   - bracketed IPv6 `PeerConfig.Address`;
   - IPv6 peer identity.
3. Enable a backend's IPv6 capability as soon as that backend can genuinely
   dial the harness IPv6 peer set. Existing BIP157/BIP158 failures should then
   score normally in the IPv6 matrix instead of keeping IPv6 capability-skipped.
4. Run every backend over:
   - `--environment ipv6 --address-lab linux-iproute`;
   - `--require-distinct-identities`;
   - the full scenario catalog, not only the smoke row.
5. Add empirical bad-peer rows over IPv6 for adapters that claim IPv6 support.
   A bad peer must be distinguishable by IPv6 identity, not only by port.

Backend tasks:

- Neutrino:
  - Keep using `neutrino.Config.ConnectPeers`, but add tests proving bracketed
    IPv6 addresses are passed through unchanged.
  - If Neutrino rejects the address or dials with the wrong network, set
    `neutrino.Config.Dialer` to a direct `net.Dialer` that handles IPv4 and
    IPv6 explicitly.
  - Once the adapter handshakes with the IPv6 peer, mark `ipv6` supported in
    `/capabilities` and let the existing long-chain header-sync failure score
    as a normal IPv6 failure until that separate issue is fixed.
- Kyoto:
  - Keep parsing peers as `SocketAddr`; bracketed IPv6 should parse in the
    adapter, so first add direct parser/config tests.
  - Run Kyoto against the IPv6 address lab. If Kyoto's builder or peer manager
    assumes IPv4 internally, patch Kyoto or the adapter wrapper so
    `TrustedPeer::from_socket_addr` receives and dials the IPv6 socket address.
  - After handshake and explicit-peer sync work, mark `ipv6` supported in
    `/capabilities`.
- Nakamoto:
  - Remove the adapter's IPv4-only domain restriction for IPv6 runs. The
    current `config.domains = vec![Domain::IPV4]` must become environment
    aware and include the Nakamoto IPv6 domain when `environment.id=ipv6`.
  - Keep `connect` populated from parsed `SocketAddr` values and add tests for
    bracketed IPv6 parsing.
  - Run the IPv6 matrix after the existing Nakamoto startup behavior is
    understood; if it still requests genesis filters or fails to reach height
    2005, classify that as the existing sync blocker, not as an IPv6 transport
    skip.
- Wasabi:
  - Extend the adapter's `PeerConfig` and `EnvironmentConfig` models so
    address type, transport, proxy address, and identity are not dropped during
    JSON deserialization.
  - Keep using `IPEndPoint.TryParse` for bracketed IPv6 and add parser tests
    for `[fd7a:b157:b158::N]:port`.
  - Verify that `Node.Connect(Network.RegTest, endpoint)` opens an IPv6 socket
    against peerlab. If NBitcoin requires extra connection parameters for IPv6,
    add them in the adapter wrapper rather than weakening the harness row.
  - After handshake and explicit-peer sync work, mark `ipv6` supported in
    `/capabilities`.

### Tor v3 Chutney Lab

Completed:

- `torlab` starts an isolated Chutney `hs-v3-min` network for a harness run.
- The harness creates one ephemeral v3 onion service per peerlab listener with
  Tor control `ADD_ONION`, advertises onion peer addresses, and tears services
  down with `DEL_ONION`.
- `PeerConfig.ProxyAddress` and `Environment.ProxyAddress` carry the Chutney
  SOCKS endpoint to adapters.
- The fake adapter passed the full Tor v3 matrix through onion peer
  identities.

Remaining:

1. Teach real adapters to use the supplied SOCKS endpoint or their native
   onion transport before enabling Tor support in their capabilities.
2. Add Tor-specific temporary-network scenarios:
   - stop and restart one onion service while the Tor network stays up;
   - stop and restart the Chutney client SOCKS endpoint;
   - delay onion reachability before adapter start;
   - verify recovery without accepting bad filter data or needing restart.
3. Run the full Tor v3 matrix for each adapter after it claims support.

### I2P and cjdns

Remaining:

1. Replace the current manifest-only rows with active router or namespace
   runtimes.
2. Provide distinct overlay identities to peerlab and adapter configuration.
3. Run the fake-adapter proof before enabling real adapter capability claims.

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
