# BIP157/BIP158 Conformance Test Suite Plan

Date: 2026-05-23

## Goal

Build a reproducible regtest conformance suite for BIP157/BIP158 light-client implementations.

The suite should validate an implementation through a small externally supplied binary. The binary wraps the implementation under test and exposes a fixed API to the test runner. The runner creates regtest chains, honest and adversarial BIP157 peers, network faults, wallet scripts, and expected results. At the end it emits a machine-readable and human-readable conformance report:

- `green`: all `MUST` requirements and all selected `SHOULD` requirements pass.
- `orange`: all `MUST` requirements pass, but one or more `SHOULD` requirements are missing or fail.
- `red`: any `MUST` requirement fails, the client misses required wallet activity under a non-adversarial setup, accepts invalid mandatory data, crashes, deadlocks, or requires a restart to recover from ordinary temporary faults.

This is a black-box suite: the tested implementation is not linked into the harness.

## Non-Goals

- Do not require the tested client to be a full node.
- Do not require full block validation by the tested client beyond what BIP157/BIP158 needs.
- Do not require a single implementation language.
- Do not rely on Bitcoin Core's honest `peerblockfilters` behavior for adversarial scenarios; custom peers are needed.
- Do not treat unimplemented `SHOULD` behavior as `red` unless it causes a mandatory correctness failure.

## High-Level Architecture

The project should have these executables:

1. `bip157-harness`
   - Owns the test run.
   - Starts/stops regtest infrastructure.
   - Drives the tested implementation through the adapter API.
   - Runs scenarios and computes score.

2. `bip157-peerlab`
   - Custom Bitcoin P2P peer simulator.
   - Speaks regtest Bitcoin P2P with enough protocol support for BIP157 clients.
   - Can act as honest peer, stale-chain peer, slow peer, malformed peer, equivocating peer, or filter liar.

3. `bip157-chainlab`
   - Builds deterministic regtest chains and expected BIP158 filters.
   - Can either use Bitcoin Core `bitcoind -regtest` as the block producer or produce simple valid regtest blocks itself later.
   - Initial version should use Bitcoin Core for correctness and speed of development.

4. `bip157-report`
   - Produces JSON, JUnit XML, and Markdown reports from a run artifact directory.

The tested implementation supplies:

1. `client-adapter`
   - A binary launched by the harness.
   - Exposes the fixed gRPC API below over Unix domain socket or TCP loopback.
   - Internally starts and controls the library under test.
   - Connects the client to the harness-controlled regtest peers.

## Proposed Repository Layout

```text
/home/user/bip157-bip158-test
  flake.nix
  flake.lock
  README.md
  PROJECT_PLAN.md
  proto/
    bip157test.proto
  crates/
    harness/
    peerlab/
    chainlab/
    report/
    common/
  adapters/
    kyoto/
      README.md
      Cargo.toml
      src/main.rs
    neutrino/
      README.md
      go.mod
      main.go
  scenarios/
    manifest.toml
    must/
    should/
    adversarial/
    network/
  docs/
    adapter-guide.md
    scoring.md
    scenario-authoring.md
    bip-coverage.md
  test-vectors/
    bip158/
    generated/
```

Rust is a good fit for the harness and peer simulator because `rust-bitcoin` provides transaction/block/message primitives and the Kyoto adapter can share the same ecosystem. The Neutrino adapter should be Go because it wraps a Go library. The protocol boundary keeps this language-neutral.

## Adapter API

Use protobuf plus gRPC. Keep the API intentionally small but explicit. The harness should launch the adapter as a child process and pass:

- adapter listen address or Unix socket path
- isolated data directory
- regtest network
- initial peer list
- log directory

The adapter must not mine blocks or directly inspect harness internals.

Initial `proto/bip157test.proto`:

```proto
syntax = "proto3";

package bip157test.v1;

service ClientUnderTest {
  rpc Capabilities(CapabilitiesRequest) returns (CapabilitiesResponse);
  rpc Configure(ConfigureRequest) returns (ConfigureResponse);
  rpc Start(StartRequest) returns (StartResponse);
  rpc Stop(StopRequest) returns (StopResponse);
  rpc Reset(ResetRequest) returns (ResetResponse);

  rpc AddPeer(AddPeerRequest) returns (AddPeerResponse);
  rpc RemovePeer(RemovePeerRequest) returns (RemovePeerResponse);
  rpc ListPeers(ListPeersRequest) returns (ListPeersResponse);
  rpc IsPeerBanned(IsPeerBannedRequest) returns (IsPeerBannedResponse);

  rpc WatchScript(WatchScriptRequest) returns (WatchScriptResponse);
  rpc Rescan(RescanRequest) returns (RescanResponse);
  rpc GetMatches(GetMatchesRequest) returns (GetMatchesResponse);

  rpc GetBestBlock(GetBestBlockRequest) returns (BlockRef);
  rpc GetBlockHashAtHeight(GetBlockHashAtHeightRequest) returns (BlockHashResponse);
  rpc GetFilterHeaderAtHeight(GetFilterHeaderAtHeightRequest) returns (FilterHeaderResponse);

  rpc WaitForSync(WaitForSyncRequest) returns (WaitForSyncResponse);
  rpc WaitForEvent(WaitForEventRequest) returns (stream ClientEvent);
  rpc Health(HealthRequest) returns (HealthResponse);
}

message CapabilitiesRequest {}

message CapabilitiesResponse {
  string implementation_name = 1;
  string implementation_version = 2;
  repeated string supported_filter_types = 3;
  bool supports_peer_ban_query = 4;
  bool supports_peer_disconnect = 5;
  bool supports_filter_header_query = 6;
  bool supports_rescan_from_height = 7;
  bool supports_persistent_storage = 8;
  bool supports_watch_after_sync = 9;
}

message ConfigureRequest {
  string network = 1;          // must be "regtest"
  string data_dir = 2;
  repeated PeerConfig peers = 3;
  uint32 required_peers = 4;
  repeated BlockRef trusted_checkpoints = 5;
  bool allow_discovery = 6;    // default false for deterministic tests
  uint64 timeout_ms = 7;
}

message ConfigureResponse {}

message StartRequest {}
message StartResponse {}
message StopRequest {}
message StopResponse {}

message ResetRequest {
  bool delete_storage = 1;
}
message ResetResponse {}

message PeerConfig {
  string id = 1;
  string address = 2;          // host:port
  bool trusted = 3;
}

message AddPeerRequest { PeerConfig peer = 1; }
message AddPeerResponse {}
message RemovePeerRequest { string peer_id = 1; }
message RemovePeerResponse {}

message ListPeersRequest {}
message ListPeersResponse {
  repeated PeerState peers = 1;
}

message PeerState {
  string id = 1;
  string address = 2;
  bool connected = 3;
  bool banned = 4;
  string last_error = 5;
  uint32 best_height = 6;
  string best_hash_hex = 7;
}

message IsPeerBannedRequest { string peer_id = 1; }
message IsPeerBannedResponse {
  bool supported = 1;
  bool banned = 2;
}

message WatchScriptRequest {
  string script_pubkey_hex = 1;
  uint32 start_height = 2;
}
message WatchScriptResponse {}

message RescanRequest {
  uint32 start_height = 1;
  uint32 stop_height = 2;      // 0 means current tip
}
message RescanResponse {}

message GetMatchesRequest {
  string script_pubkey_hex = 1;
  uint32 start_height = 2;
  uint32 stop_height = 3;      // 0 means current tip
}

message GetMatchesResponse {
  repeated TxMatch matches = 1;
}

message TxMatch {
  string txid_hex = 1;
  string block_hash_hex = 2;
  uint32 height = 3;
  MatchKind kind = 4;
  uint32 vout = 5;
  uint32 vin = 6;
}

enum MatchKind {
  MATCH_KIND_UNSPECIFIED = 0;
  MATCH_KIND_OUTPUT = 1;
  MATCH_KIND_SPEND = 2;
}

message GetBestBlockRequest {}

message BlockRef {
  string hash_hex = 1;
  uint32 height = 2;
}

message GetBlockHashAtHeightRequest { uint32 height = 1; }
message BlockHashResponse {
  bool found = 1;
  string hash_hex = 2;
}

message GetFilterHeaderAtHeightRequest { uint32 height = 1; }
message FilterHeaderResponse {
  bool supported = 1;
  bool found = 2;
  string filter_header_hex = 3;
}

message WaitForSyncRequest {
  string target_hash_hex = 1;
  uint32 target_height = 2;
  uint64 timeout_ms = 3;
}

message WaitForSyncResponse {
  bool reached = 1;
  BlockRef best = 2;
  string error = 3;
}

message WaitForEventRequest {
  uint64 timeout_ms = 1;
}

message ClientEvent {
  EventKind kind = 1;
  BlockRef block = 2;
  TxMatch match = 3;
  string peer_id = 4;
  string message = 5;
}

enum EventKind {
  EVENT_KIND_UNSPECIFIED = 0;
  EVENT_KIND_SYNCED = 1;
  EVENT_KIND_BLOCK_CONNECTED = 2;
  EVENT_KIND_BLOCK_DISCONNECTED = 3;
  EVENT_KIND_TX_MATCH = 4;
  EVENT_KIND_PEER_CONNECTED = 5;
  EVENT_KIND_PEER_DISCONNECTED = 6;
  EVENT_KIND_PEER_BANNED = 7;
  EVENT_KIND_WARNING = 8;
}

message HealthRequest {}
message HealthResponse {
  bool alive = 1;
  string status = 2;
}
```

Adapter requirements:

- Must start from an empty data directory on `Reset(delete_storage=true)`.
- Must never connect to uncontrolled external peers when `allow_discovery=false`.
- Must expose best-known block hash and height.
- Must expose wallet matches for watched scripts.
- Should expose filter headers and peer ban state. If not available, tests depending on those APIs are marked `unsupported-should` unless mandatory correctness can still be verified externally.

## Harness Chain Model

Use a deterministic regtest fixture builder that creates:

- coinbase outputs to watched scripts
- non-coinbase outputs to watched scripts
- spends of watched outputs
- unrelated transactions
- OP_RETURN outputs
- empty-script outputs
- P2PK, P2PKH, P2SH, P2WPKH, P2WSH, P2TR outputs
- legacy, segwit v0, nested segwit, and taproot spends
- blocks with zero-element basic filters when possible
- reorgs and stale branches

For each block, `chainlab` stores:

- block bytes
- header
- height
- canonical status
- expected BIP158 basic filter bytes
- filter hash
- filter header
- expected wallet matches
- prevout script set used to construct the exact filter

The authoritative filter builder must be independent of the implementation under test. It should include:

- BIP158 `P = 19`
- BIP158 `M = 784931`
- key `k = first 16 bytes of block hash`
- CompactSize-prefixed serialization
- zero-element filter as one byte of zero
- all non-OP_RETURN output scripts, including coinbase outputs
- all non-coinbase input prevout scripts
- no nil items

## Peer Simulator

`peerlab` must implement enough Bitcoin P2P for BIP157 clients:

- version/verack
- sendheaders
- ping/pong
- getheaders/headers
- getdata/block
- getcfilters/cfilter
- getcfheaders/cfheaders
- getcfcheckpt/cfcheckpt
- optional addr/addrv2 responses, but deterministic tests should use explicit peers and discovery off

Each peer is configured by scenario:

```toml
[[peers]]
id = "honest-a"
behavior = "honest"
chain = "main"
latency_ms = 0
disconnect_after = ""

[[peers]]
id = "liar-filter-header"
behavior = "bad_filter_header_at_height"
height = 125
bad_filter_mode = "omit_watched_output"
```

Peer behavior modules:

- honest
- no compact-filter service bit
- claims service bit but ignores BIP157 requests
- serves unknown/stale stop hashes incorrectly
- serves too many filter hashes
- wrong filter type
- wrong stop hash
- wrong previous filter header
- wrong number of filter hashes
- self-consistent fake filter-header chain
- conflict at selected height
- malformed GCS bytes
- filter hash does not match served filter
- filter omits watched output
- filter omits watched spend prevout
- filter omits coinbase output
- filter includes OP_RETURN
- stale branch headers with less work
- heavier fork after initial sync
- slow responses
- intermittent disconnects
- equivocation across reconnects

## Network Fault Injection

Use two layers.

Layer 1: deterministic peer behavior inside `peerlab`.

- delayed response for selected message types
- dropped response for selected message types
- disconnect before/after selected response
- connection refusal windows
- half-open connection behavior if practical
- reordered filter/filter-header responses

Layer 2: Linux network namespace and `tc netem`, modeled after `/home/user/arti/guards-banned-forever/demo/client-outage-repro`.

The outage repro uses a client-only network namespace with a veth pair and applies `tc netem` delay to the client link. This suite should adapt that pattern:

- run adapter process in a network namespace
- put peer simulator and bitcoind on the host-side namespace
- apply base latency, spike latency, packet loss, duplication, and reordering with `tc qdisc netem`
- remove impairment and verify the client recovers without restart

Fault profiles:

- baseline latency: 100-500 ms, no loss
- short outage: all traffic delayed/dropped for 5-15 seconds
- long outage below test timeout: 30-90 seconds
- burst loss: 20-60% packet loss for selected windows
- asymmetric fault: delay client-to-peer only or peer-to-client only
- slow peer mixed with healthy peer
- all peers temporarily unreachable, then restored
- peer disconnects during filter-header sync
- peer disconnects during filter download
- peer disconnects during block download after a filter match

Recovery assertions:

- client process stays alive
- health API remains responsive or recovers
- client reconnects or selects another peer
- client reaches current tip after impairment is removed
- client does not permanently ban honest peers solely due to client-side outage
- client does not require data-dir deletion or restart
- no duplicate or missing wallet matches after recovery

## Scoring Model

Each test case records:

- `requirement_id`
- BIP reference
- requirement level: `MUST`, `SHOULD`, `MAY`, or `IMPLEMENTATION`
- result: `pass`, `fail`, `unsupported`, `skipped`, `timeout`
- severity: `red`, `orange`, `info`
- evidence: logs, packets, peer transcript, adapter events, final chain state

Suite color:

- `red` if any `MUST` fails, any mandatory honest-network wallet match is missed, an invalid BIP158 filter is accepted as correct under mandatory conditions, the adapter crashes, or recovery requires restart after temporary faults.
- `orange` if all `MUST` pass but any `SHOULD` fails or is unsupported.
- `green` if all `MUST` and selected `SHOULD` pass.

The report must include a table of every BIP157/BIP158 requirement considered, including tests not yet implemented.

## Test Matrix

### BIP158 Mandatory Filter Construction

These are red on failure when the client accepts bad data or misses required wallet activity under honest peers.

1. Basic output inclusion
   - watched script in non-coinbase output
   - expected: client reports receive transaction

2. Coinbase output inclusion
   - watched script in coinbase output
   - expected: client reports receive transaction after maturity policy if the client cares about spendability, but at minimum detects the output

3. OP_RETURN exclusion
   - peer serves filter including OP_RETURN and causing header mismatch
   - expected: exact filter/header validation identifies this as wrong during conflict resolution

4. Nil/empty script exclusion
   - block with empty script output
   - expected: exact BIP158 filter excludes it

5. Prevout script inclusion: legacy P2PKH
   - watched script is spent by legacy input
   - expected: client detects spend

6. Prevout script inclusion: P2SH
   - expected: client detects spend

7. Prevout script inclusion: P2WPKH
   - expected: client detects spend

8. Prevout script inclusion: P2WSH
   - expected: client detects spend

9. Prevout script inclusion: P2TR
   - expected: client detects spend

10. Zero-element filter serialization
    - block with no filter elements
    - expected: accepted only if serialized as one zero byte and committed correctly

11. Parameter enforcement
    - peer serves filter with wrong `P`, wrong `M`, or wrong key-derived contents
    - expected: rejected because it does not link to expected filter header

12. BIP158 test vectors
    - import official five testnet vectors as unit tests for `chainlab` builder
    - expected: fixture builder matches vectors exactly

### BIP157 Message and Chain Behavior

1. Headers-first sync
   - peers refuse BIP157 until headers are available
   - expected: client syncs headers before relying on filters

2. Longest proof-of-work chain
   - one peer serves lower-work chain, one serves higher-work chain
   - expected: client follows higher-work chain and disconnects or ignores low-work peer

3. Trusted checkpoint start
   - client starts from configured checkpoint
   - expected: sync starts at or after checkpoint and reaches tip

4. Filter-header derivation
   - honest peer sends filter hashes and previous filter header
   - expected: client derives/stores correct filter headers

5. Filter-header checkpoint range
   - peers serve checkpoint headers every 1000 blocks
   - expected: client verifies ranges against checkpoints when it uses `getcfcheckpt`

6. Filter linkage
   - cfilter hash links to stored filter header
   - expected: accepted

7. Bad cfilter linkage
   - cfilter does not link to stored header
   - expected: rejected and peer banned/suppressed

8. Unknown or stale StopHash handling from peer
   - peer sends responses for unrequested or wrong stop hash
   - expected: ignored or punished according to severity

9. Maximum range enforcement
   - peer sends more than 1000 filters or more than 2000 filter hashes/checkpoints
   - expected: rejected

10. New valid block header
    - after initial sync, mine a block
    - expected: client requests/accepts corresponding filter header from eligible peers and detects wallet matches

11. Filter-header persistence
    - sync, stop adapter, restart with same data dir, introduce peer disagreement for recent headers
    - expected: client compares recent persisted headers and detects conflict

### Adversarial Scenarios

1. Single malicious peer, no trusted checkpoint
   - self-consistent bad filter headers and filters omit watched output
   - expected: if implementation is configured to trust one peer, mark orange for missing multi-peer SHOULD; if it misses wallet transaction under claimed conformance, record failure according to declared trust model

2. One honest, one malicious: conflicting filter headers
   - first disagreement at height `h`
   - expected: client identifies `h`, downloads block, derives correct filter/header, bans malicious peer

3. One honest, one malicious: malicious replies first
   - expected: same as above, no permanent acceptance based solely on arrival order

4. Honest majority, malicious minority
   - expected: malicious peers banned/suppressed

5. Malicious majority, honest minority
   - expected: client should not blindly majority-ban honest peer if it can derive exact filter; if it cannot, orange or red depending on false-negative outcome

6. All peers malicious but mutually consistent
   - expected: without trusted checkpoints/external source, this is an eclipse limitation; report as `known-unverifiable`, but verify the client does not claim stronger security than configured

7. Filter matches header but omits coinbase output
   - malicious filter-header chain commits to invalid filter
   - expected: conflict resolution catches if any honest peer exists; exact BIP158 verifier must include coinbase output

8. Filter omits prevout script for taproot spend
   - expected: exact verifier catches if resolving conflict from block plus prevout set

9. Bad peer stalls after sending conflicting header
   - expected: client does not deadlock; identifies unresponsive peer or retries

10. Peer equivocation
    - same peer sends different filter headers across reconnects
    - expected: client detects and suppresses/bans peer

11. Reorg during filter sync
    - expected: client rolls back stale filter headers and filters, then syncs new branch

12. Stale branch block fetch
    - request block/hash on stale branch if adapter exposes it
    - expected: client API clearly distinguishes canonical vs known-stale hashes

### Network Fault Scenarios

1. Temporary full outage after header sync
2. Temporary full outage during header sync
3. Temporary full outage during filter-header sync
4. Temporary full outage during filter download
5. Temporary full outage during block download after a positive filter match
6. Slow malicious peer plus fast honest peer
7. Fast malicious peer plus slow honest peer
8. Packet loss with multiple peers
9. Peer connection churn while new blocks are mined
10. Adapter restart test: only for persistence scenarios, not as recovery from ordinary faults

Each network test must assert recovery without restart unless the scenario explicitly tests persistence across restart.

## Upstream Issue-Derived Scenario Backlog

This section records the open-issue sweep performed on 2026-05-23. Added
scenarios are either direct BIP157/BIP158 coverage or implementation/stress
checks that do not contradict the BIPs. Non-BIP scenarios must be scored as
`IMPLEMENTATION` or `MAY` unless they cause a mandatory BIP157/BIP158 failure.

### Neutrino-Derived Additions

| Source | Scenario to Add | BIP Alignment | Covered Yet |
| --- | --- | --- | --- |
| https://github.com/lightninglabs/neutrino/issues/349 | `network.followup_nonresponse_not_bad_data`: during filter-header conflict resolution, make an honest peer answer the first request but drop or delay the follow-up `cfheaders`/`cfilter` query. The client may retry, disconnect, or deprioritize, but must not report that peer as proven invalid solely because it did not answer. | Supports BIP157's bad-peer interrogation recommendation without treating network failure as cryptographic evidence. | Not yet covered; current delay tests check recovery, not false-ban classification during conflict follow-up. |
| https://github.com/lightninglabs/neutrino/issues/338 | `bip157.stop_hash_known_by_peer`: create a reorg/stale-branch setup where peer A has seen a stop hash and peer B has not. Assert the client only sends `getcfilters`/`getcfheaders` to peers that know the stop hash, or handles the `SHOULD NOT respond` case without reconnect loops. | Direct BIP157 message-contract coverage. | Partially listed as unknown/stale StopHash handling; not implemented as a peer-specific knowledge test. |
| https://github.com/lightninglabs/neutrino/issues/218 | `bip157.invalid_persisted_filter_headers_recover`: first sync from mutually consistent malicious peers that commit to a bad filter-header chain, persist state, restart, then introduce an honest peer and require rollback/resync rather than deadlock. | Extends BIP157 conflict-resolution and filter-header persistence coverage. | Not covered; current self-consistent eclipse scenario only reports the trust limitation. |
| https://github.com/lightninglabs/neutrino/issues/315 and https://github.com/lightninglabs/neutrino/issues/276 | `storage.partial_write_recovery`: after an honest sync, corrupt/truncate local header or filter-header storage in a controlled way, restart, and require either automatic repair/resync or a precise non-crashing error. | Implementation durability; not a BIP conformance requirement. | Not covered. |
| https://github.com/lightninglabs/neutrino/issues/253 | `ban.expiry_and_unban_capability`: after a provable bad peer is banned, verify ban state is observable when supported, expires or can be cleared according to implementation policy, and the peer is not retried before that policy allows. | Implementation peer-management check; BIP157 recommends banning provably bad peers but does not define an unban API. | Partially covered by ban-state assertions; expiry/unban is not covered. |
| https://github.com/lightninglabs/neutrino/issues/110, https://github.com/lightninglabs/neutrino/issues/4, and https://github.com/lightninglabs/neutrino/pull/334 | `network.idle_keepalive_and_long_wait`: keep the client connected while no new blocks or filters arrive for longer than ordinary peer idle timeouts, then mine a new block and require continued sync without restart. | Implementation liveness; compatible with BIP157. | Not covered; current outages are active fault windows, not idle waits. |
| https://github.com/lightninglabs/neutrino/issues/292 and https://github.com/lightninglabs/neutrino/pull/303 | `query.heterogeneous_peer_work_stealing`: run many concurrent filter/block requests across fast, slow, and intermittently failing peers; require progress, bounded latency, and no starvation of healthy peers. | Implementation performance/liveness; compatible with the BIP157 multi-peer model. | Partially covered by slow/fast peer scenarios; no high-volume job queue yet. |
| https://github.com/lightninglabs/neutrino/issues/60 | `config.invalid_initial_peer_fallback`: configure multiple peers where one DNS/address entry is invalid and at least one honest peer is valid. The adapter/client should continue with valid peers rather than fail the entire run. | Implementation robustness; compatible with BIP157's multiple-peer recommendation. | Not covered. |
| https://github.com/lightninglabs/neutrino/issues/6 and https://github.com/lightninglabs/neutrino/issues/67 | `chain.deep_reorg_sendheaders`: after initial sync, deliver a deep reorg and new tips via `sendheaders`; include a restart between old and new branches. The client must follow most work, roll back stale filter state, and resume. | Headers-first and most-work behavior are BIP157-aligned; `sendheaders` itself is BIP130 and optional. | Partially covered by generic reorg during filter sync; not deep, restarted, or sendheaders-specific. |
| https://github.com/lightninglabs/neutrino/issues/71, https://github.com/lightninglabs/neutrino/issues/66, and https://github.com/lightninglabs/neutrino/pull/282 | `chain.long_checkpointed_header_sync`: build a long chain crossing multiple BIP157 checkpoint intervals and run header/filter-header sync with slow peers, large batches, and repeated `getcfcheckpt` use. | Directly exercises BIP157 range/checkpoint behavior; performance assertions are implementation-level. | Not covered; current fixture is short and this is the next likely blocker for Neutrino. |
| https://github.com/lightninglabs/neutrino/pull/345 | `import.cfilter_header_linkage`: for clients that expose an import path, import valid and invalid compact filters from a non-P2P source and require verification against stored filter headers before persistence. | Implementation extension of BIP157 filter-header linkage; not required for black-box P2P clients. | Not covered and should be optional. |
| https://github.com/lightninglabs/neutrino/pull/340 | `rescan.concurrent_watch_and_cancel`: add watch scripts before sync, during sync, and after sync; cancel/restart rescans; require deduplicated wallet matches and no missed spends. | Wallet-client correctness built on BIP158 filters; not a BIP wire requirement. | Partially covered by simple watch/match; not concurrent or cancellation-heavy. |
| https://github.com/lightninglabs/neutrino/issues/319 | `transport.bip324_optional`: when an adapter declares BIP324/v2transport support, rerun the honest and adversarial matrix over encrypted transport. | Optional transport coverage; BIP157 does not require BIP324. | Not covered and should be skipped unless supported. |

Open Neutrino items intentionally not added as BIP157/BIP158 suite work:
`#350` mempool support, `#332` standalone REST API, `#252` build docs, `#237`
WASM, `#224` logo, `#142` Litecoin, `#69` BIP151, `#64` witness-download
optimization, `#9` transaction broadcast, `#89` internal peer iteration cleanup,
and old generic test flakes `#304`, `#169`, `#30`, `#29` except where their
liveness theme is covered by the scenarios above.

Open Neutrino PRs intentionally not treated as BIP157/BIP158 conformance
coverage: SQL backend/migration stack `#352`-`#357` is covered only through the
storage durability scenario if an adapter exposes those backends; fee estimator
`#351`, custom chainimport HTTP client `#343`, query submodule refactor `#325`,
code cleanup `#312`, logging `#291`, transaction relay `#190`, and reject
message plumbing `#111` are outside this suite's compact-filter scope. PR
`#348` is excluded from the findings because it is about Tor v3 banman address
recognition, not compact-filter bad-data classification.

### Kyoto-Derived Additions

| Source | Scenario to Add | BIP Alignment | Covered Yet |
| --- | --- | --- | --- |
| https://github.com/2140-dev/kyoto/issues/561 | `chain.reorg_filter_in_flight_long`: mine a chain long enough to require several filter requests, start sync, invalidate a block, mine a heavier branch, and deliver stale/in-flight filters around the reorg. Assert no permanent hang, no unjustified ban of the only honest peer, and correct rollback of filter headers/filters. | Directly supports BIP157 most-work and filter-header consistency expectations. | Partially covered by generic reorg during filter sync; not the long/racy 4200-block style case. |
| https://github.com/2140-dev/kyoto/issues/538 | `api.sync_phase_observability`: optional adapter capability for headers-only sync, filter-header/filter sync, and matched-block download phases, with progress events for each. | API/testability enhancement; does not change BIP157 requirements. | Not covered; current API observes best block and matches only. |
| https://github.com/2140-dev/kyoto/issues/583 | `checkpoint.network_specific_methods`: verify adapters do not rely on mainnet-only checkpoint helpers in regtest/signet-style runs and return explicit unsupported errors for network-specific checkpoint APIs. | Implementation API robustness; compatible with BIP157 trusted checkpoints. | Not covered. |
| https://github.com/2140-dev/kyoto/pull/556 | no BIP157/BIP158 scenario | Transaction broadcast timeout behavior is outside the compact-filter conformance scope. | Not applicable. |

Open Kyoto items intentionally not added as black-box conformance scenarios:
`#578` rbmt migration and `#443` crate restructuring are internal architecture
work with no direct BIP157/BIP158 behavior to assert.

## Kyoto Adapter Plan

Kyoto is Rust, so build `adapters/kyoto` as a Rust binary.

Responsibilities:

- Parse gRPC requests.
- Build a Kyoto node in regtest mode.
- Configure explicit trusted peers from `ConfigureRequest.peers`.
- Disable discovery where Kyoto supports it; otherwise use whitelist-only mode for deterministic tests.
- Set `required_peers` from the request.
- Add watched scripts via Kyoto client APIs.
- Stream Kyoto events into an adapter-side event log.
- Implement `GetBestBlock`, `GetBlockHashAtHeight`, `GetMatches`, and `WaitForSync`.
- Implement `IsPeerBanned` if Kyoto exposes enough state; otherwise return `supported=false`.
- Implement `GetFilterHeaderAtHeight` if Kyoto exposes it; otherwise return `supported=false`.

Expected early Kyoto results:

- likely red/orange for adversarial multi-peer conflict resolution until Kyoto implements BIP157 interrogation/ban behavior
- likely orange for default single-peer trust unless configured with multiple peers
- useful first target because the adapter should be straightforward

Validation steps:

1. Adapter compiles in Nix shell.
2. Harness can start Kyoto adapter and one honest peer.
3. Kyoto syncs honest regtest chain.
4. Kyoto detects receive/spend matches for simple scripts.
5. Run first adversarial conflict scenario and confirm report captures current behavior.

## Neutrino Adapter Plan

Neutrino is Go, so build `adapters/neutrino` as a Go binary.

Responsibilities:

- Wrap `lightninglabs/neutrino.ChainService`.
- Use a temp walletdb/data dir from `ConfigureRequest`.
- Set `ChainParams` to regtest/simnet-compatible params. Prefer regtest if possible; if Neutrino is tightly coupled to SimNet in tests, add explicit compatibility work so the conformance suite remains regtest.
- Configure `AddPeers` only from harness peers.
- Set max peers and ban duration deterministically.
- Use `GetCFilter`, `GetBlock`, `BestBlock`, rescan APIs, and peer APIs to implement gRPC.
- Expose ban state through Neutrino `IsBanned` if available.

Expected early Neutrino results:

- should pass more conflict-resolution tests than Kyoto
- likely fail or orange for exact BIP158 block-derived verification in coinbase-output and unsupported-prevout cases
- direct bad `cfilter` ban behavior may score orange if it ignores rather than bans bad direct responses

Validation steps:

1. Adapter compiles in Nix shell.
2. Harness can start Neutrino adapter and one honest regtest peer.
3. Neutrino syncs honest regtest chain.
4. Neutrino detects simple receive/spend matches.
5. Run conflict-resolution scenarios already analogous to Neutrino's internal tests, but through real P2P peer simulators.

## Reproducibility Plan

Use Nix flakes.

Pinned dependencies:

- Rust toolchain
- Go toolchain
- protobuf compiler
- gRPC generators for Rust and Go
- Bitcoin Core
- iproute2 for `ip` and `tc`
- nftables or iptables if needed
- jq
- just
- cargo-nextest
- gotestsum

`nix develop` should provide all tools. `nix run .#harness -- ...` should run the suite.

Because `tc netem` and network namespaces require elevated privileges or capabilities, provide two modes:

1. `basic`: no privileged netns; uses peerlab-level delays/disconnects only.
2. `netem`: uses Linux netns/veth/tc and requires root or configured capabilities.

The report must record which mode ran.

## Scenario Manifest

Use TOML or YAML for scenario metadata, but compile core scenario logic in Rust for safety.

Example:

```toml
id = "bip157.conflict.one-honest-one-liar"
title = "Filter-header conflict resolved by block-derived filter"
level = "SHOULD"
bip = "BIP157"
reference = "bip-0157.mediawiki:386-392"
category = "adversarial"
requires = ["peer_ban_query", "filter_header_query"]
timeout_ms = 120000

[[peers]]
id = "honest"
behavior = "honest"

[[peers]]
id = "liar"
behavior = "bad_filter_header_at_height"
height = 125
bad_filter_mode = "omit_watched_output"

[expected]
suite_color_if_fail = "orange"
must_reach_tip = true
must_find_first_disagreement = true
must_ban = ["liar"]
must_not_ban = ["honest"]
```

## Evidence and Debuggability

Each run should produce:

- `run.json`: complete scenario results
- `summary.md`: human-readable report
- `junit.xml`: CI integration
- `peers/*.pcap` if packet capture is enabled
- `peers/*-transcript.jsonl`: all P2P messages sent/received by peerlab
- `adapter/stdout.log`, `adapter/stderr.log`
- `adapter/events.jsonl`
- `chain/blocks/`
- `chain/filters.jsonl`
- `netem.log`

On failure, the report should include:

- scenario id
- expected behavior
- observed behavior
- first relevant peer transcript entries
- adapter health and final best block
- peer ban state
- wallet match diff

## Milestones

### Milestone 0: Skeleton and API

- Add flake.
- Add proto.
- Generate Rust and Go stubs.
- Implement a fake adapter used only for harness development.
- Implement report schema and color rules.

Exit criteria:

- `nix develop` works.
- `nix run .#harness -- --adapter fake` runs one passing scenario.

### Milestone 1: Honest Regtest Sync

- Implement chainlab using Bitcoin Core regtest.
- Implement honest peerlab peer serving headers, blocks, cfheaders, cfilters, cfcheckpts.
- Implement simple receive/spend fixture.
- Implement Kyoto adapter.

Exit criteria:

- Kyoto syncs from one honest peer.
- Kyoto reports expected receive/spend for a watched P2WPKH script.
- Report produces green for honest minimal scenario.

### Milestone 2: BIP158 Exactness

- Implement independent BIP158 filter builder.
- Validate builder against official BIP158 vectors.
- Add coinbase output, OP_RETURN exclusion, empty filter, and prevout-spend scenarios.

Exit criteria:

- Chainlab filter builder matches official vectors.
- Honest peer scenarios pass with Kyoto/Neutrino where their public APIs support the needed observation.

### Milestone 3: Filter Header Conflict Harness

- Add two-peer and three-peer adversarial scenarios.
- Add first-disagreement detection checks.
- Add peer ban/suppression assertions.
- Add self-consistent bad-chain scenario.

Exit criteria:

- Harness can reproduce Kyoto's current disconnect-without-ban behavior.
- Harness can exercise Neutrino's current conflict-resolution behavior through real P2P peers, not internal mocks.

### Milestone 4: Neutrino Adapter

- Implement Go adapter.
- Make regtest peer connection deterministic.
- Map Neutrino rescan/match APIs to the proto.
- Expose ban state.

Exit criteria:

- Neutrino passes honest sync and wallet match scenarios.
- Neutrino runs adversarial filter-header scenarios and report records pass/orange/red accurately.

### Milestone 5: Network Faults

- Implement peerlab-level delays/drops/disconnects.
- Implement optional netns/veth/tc runner based on the Arti outage repro pattern.
- Add recovery assertions.

Exit criteria:

- Client recovers from outage during headers, filter headers, filters, and block fetch without restart.
- Honest peers are not permanently punished for client-side outage.

### Milestone 6: Coverage Completion

- Add BIP coverage table.
- Add every BIP157/BIP158 MUST/SHOULD item as implemented, unsupported, or out of scope.
- Add CI job for basic mode.
- Add privileged/manual job documentation for netem mode.

Exit criteria:

- A new implementation author can read docs, write an adapter, and run the suite.
- Report clearly states conformance color and detailed reasons.

## Implementation Guide Requirements

`docs/adapter-guide.md` must explain how to wrap any implementation:

1. Build a binary that starts a gRPC server implementing `ClientUnderTest`.
2. Accept data dir and peer list from `Configure`.
3. Ensure the client only connects to provided regtest peers.
4. Convert library events into `ClientEvent`.
5. Implement script watching and match reporting.
6. Implement best-block and block-hash queries.
7. Expose ban state if possible.
8. Cleanly stop and reset.
9. Return explicit `unsupported` fields for features the library cannot expose.

The guide must include:

- Rust/Kyoto example
- Go/Neutrino example
- generic pseudocode for other languages
- expected process lifecycle
- timeout behavior
- logging expectations
- common pitfalls

## Open Design Questions

1. Should the suite require the adapter to expose raw filter headers, or should peer transcript plus wallet outcomes be enough for clients that do not expose internals?
2. Should failure to expose ban state make every ban-related `SHOULD` orange, or should the suite infer suppression through reconnect behavior when possible?
3. Should the initial custom peer use Bitcoin Core block data only, or should chainlab mine blocks itself after Milestone 2?
4. How strict should "download full block from random outbound peers" be tested? Packet transcripts can detect which peer served the block, but statistical randomness requires repeated runs.
5. How should trusted-peer mode be represented so single-peer deployments can be scored fairly without weakening the general conformance result?

## Immediate Next Steps

1. Write `proto/bip157test.proto`.
2. Create a minimal Nix flake with Rust, Go, protobuf, Bitcoin Core, and iproute2.
3. Implement fake adapter and one harness smoke test.
4. Implement chainlab fixture builder for a short honest chain.
5. Implement honest peerlab with headers, cfheaders, cfilters, and blocks.
6. Build the Kyoto adapter first.
7. Run honest sync and first adversarial conflict against Kyoto.
8. Build the Neutrino adapter.
9. Add netem mode after the basic adversarial suite is stable.
