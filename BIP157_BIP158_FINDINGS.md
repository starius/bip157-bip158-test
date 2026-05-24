# BIP157/BIP158 Findings

This file records implementation mismatches and limitations found while
building the conformance suite. `MUST` failures should score red. Missing or
weak `SHOULD` behavior should score orange unless it causes a mandatory
correctness failure.

## Kyoto

`KYOTO-SHOULD-001` (`SHOULD`): a self-consistent malicious peer set can
supply matching bad filter headers and bad filters without a block-derived
recomputation check detecting the lie. This is a trust-limit case, but it must
be reported as such rather than described as full BIP158 validation.

Evidence: filter events are accepted from received cfilters after header-chain
checks; the public API exposes matched filters and block fetches, not a full
local filter derivation path. See `2140-dev/kyoto:src/chain/chain.rs` and
`2140-dev/kyoto:src/client.rs`.

Coverage: `bip157.self_consistent_eclipse`.

`KYOTO-SHOULD-002` (`SHOULD`): a single lying peer can cause a sync or match
scenario to fail by serving conflicting compact-filter data; Kyoto exposes
disconnect-style peer churn but not a clear persistent ban result to the
adapter API.

Evidence: block requests are sent to a random peer from the queue in
`2140-dev/kyoto:src/node.rs`. Peer state exposed by `peer_info` lists current
connections, not ban status, in `2140-dev/kyoto:src/client.rs`.

Coverage: `bip157.conflict_one_honest_one_liar` and
`bip157.direct_bad_cfilter_ban`.

`KYOTO-MUST-003` (`MUST-risk`): `get_block` accepts any known header hash,
while the user-facing contract around related helpers is current-chain
oriented. Kyoto has added a canonical-only helper for the public
`height_of_hash` path, but `get_block` and block handling still use the
broader known-header lookup.

Evidence: `height_of_hash_canonical_only` exists in
`2140-dev/kyoto:src/chain/graph.rs`; `get_block` and received-block indexing
still use the broader `height_of_hash` path in `2140-dev/kyoto:src/node.rs`.

Coverage: `kyoto.stop_reorg_start_on_orphan` plus a stale-header block-request
test.

`KYOTO-SHOULD-004` (`SHOULD`): peer disagreement resolution is not yet exposed
as a strong black-box outcome. A conformant client should detect disagreement
across peers and recover via an honest peer or a block-derived filter check.

Evidence: latest matrix rows for bad `cfheaders`, bad `cfcheckpt`, bad
`cfilter`, wrong filter type, and conflict resolution fail. Some rows return
"not punished"; others return adapter `/list-peers` 503 after the bad
transcript.

Coverage: `bip157.conflict_one_honest_one_liar`,
`neutrino.cfheaders_mismatch.*`, `neutrino.cfcheckpt_sanity.*`, and
`neutrino.resolve_filter_mismatch.*`.

## Neutrino

`NEUTRINO-MUST-001` (`MUST`): `VerifyBasicBlockFilter` skips the coinbase
transaction even though BIP158 basic filters include coinbase output scripts
except unspendable outputs.

Evidence: the verifier skips index 0 in `lightninglabs/neutrino:verification.go`.

Coverage: `bip158.coinbase_output_included`.

`NEUTRINO-SHOULD-002` (`SHOULD`): filter verification cannot derive every
input prevout script from the spending transaction. Non-witness and unsupported
witness/script forms are skipped, so conflict resolution can fall back to peer
majority rather than exact filter derivation.

Evidence: unsupported or non-witness inputs are skipped in
`lightninglabs/neutrino:verification.go`; the fallback majority branch is
documented in `lightninglabs/neutrino:blockmanager.go`.

Coverage: `bip158.prevout_legacy_included`, future nested-segwit cases, and
future script-coverage cases.

`NEUTRINO-SHOULD-003` (`SHOULD`): bad-filter peer resolution is meaningful but
not complete for all BIP158 element sets because it relies on partial
verification plus OP_RETURN mismatch heuristics.

Evidence: OP_RETURN mismatch counting and potential bans are in
`lightninglabs/neutrino:blockmanager.go`; unit coverage exists in
`lightninglabs/neutrino:bamboozle_unit_test.go`.

Coverage: `bip157.direct_bad_cfilter_ban` and
`neutrino.resolve_filter_mismatch.*`.

`NEUTRINO-TEST-004` (`Test gap`): Neutrino has deeper internal tests than
Kyoto, including real `rpctest` nodes and conflict-resolution unit tests, but
those tests are implementation-specific and not a reusable BIP157/BIP158
conformance suite.

Evidence: SimNet/rpctest setup and multi-peer setup appear in
`lightninglabs/neutrino:sync_test.go`.

Coverage: the suite catalog imports all listed Neutrino baseline scenarios and
adds stronger black-box adversarial scenarios.

`NEUTRINO-BLOCKER-005` (`Unclassified`): the strict adapter fails most P2P
scenarios before compact-filter adversarial checks become meaningful.

Evidence: latest run times out after peerlab sends the first 2000-header page
and Neutrino disconnects. Kyoto and Wasabi pass the same long-chain peerlab
rows, so this is not classified as a bad scenario.

Coverage: `adapter.honest_wallet_receive_spend`,
`chain.long_checkpointed_header_sync`, and all adapter rows that require tip
sync.

## Nakamoto

`NAKAMOTO-BLOCKER-001` (`Unclassified`): the strict adapter builds, starts, and
connects to peerlab, but does not advance to the long-chain tip in the
black-box suite. This is not yet classified as a Nakamoto implementation bug;
it needs a focused adapter/library compatibility investigation.

Evidence: latest run shows version/verack/ping traffic and some `getcfilters`
requests for regtest genesis, but no successful height-2005 sync. See
`VALIDATION_REPORT.md` and `IMPLEMENTATION_MATRIX.md`.

Coverage: `adapter.honest_wallet_receive_spend`,
`chain.long_checkpointed_header_sync`, and all adapter scenarios that require
tip sync.

## Wasabi

`WASABI-MUST-001` (`MUST`): the open P2P compact-filter PR validates a
`cfheaders` range starting at height 1 against zero rather than the genesis
filter header.

Evidence: the local patch
`nix/patches/wasabi/0002-anchor-height-one-to-genesis-filter-header.patch` is
required on top of
`nix/patches/wasabi/0001-p2p-compact-filter-provider.patch` for honest
height-1 `cfheaders` sync to pass.

Coverage: `adapter.honest_wallet_receive_spend` and
`bip157.cfheaders_order_and_checkpoint_boundaries`.

`WASABI-SHOULD-002` (`SHOULD`): the patched P2P client reaches the tip and
scans filters, but does not punish or clearly reject bad compact-filter peers
in the active black-box rows.

Evidence: latest Wasabi run fails bad `cfcheckpt`, bad `PrevFilterHeader`,
conflicting `cfheaders`, corrupt `cfilter`, wrong block hash, malformed
payload, empty `cfheaders`, wrong filter type, unresponsive peer, and
scrambled-header rows.

Coverage: `bip157.bad_cfcheckpt_response`,
`bip157.bad_cfheaders_prev_header`, `bip157.conflict_one_honest_one_liar`, and
`neutrino.resolve_filter_mismatch.*`.

`WASABI-MUST-003` (`MUST`): invalid downloaded blocks are not exposed as
rejected or punished in the strict adapter result.

Evidence: `blocks.invalid_downloaded_block_rejected` fails with
"bad-block was not punished after serving an invalid downloaded block".

Coverage: `blocks.invalid_downloaded_block_rejected`.

## Upstream Issue and PR Cross-References

Checked against open GitHub issues and PRs on 2026-05-24. These are
cross-references for future upstream reports. They are not substitutes for the
local evidence above.

### Kyoto

- `KYOTO-SHOULD-001`, `KYOTO-SHOULD-002`, and `KYOTO-SHOULD-004` overlap
  partly with https://github.com/2140-dev/kyoto/issues/561. The issue is about
  reorganization handling while filter data is in flight, including
  `UnknownFilterHash` behavior and peer banning/exhaustion. It is not a direct
  report of the self-consistent eclipse case.
- `KYOTO-SHOULD-004` is adjacent to
  https://github.com/2140-dev/kyoto/issues/538, which asks for separate
  header, filter, and block-fetch process control. That would make conformance
  adapters and reports more precise, but is not itself a BIP157/BIP158
  mismatch.
- `KYOTO-MUST-003`: no open Kyoto issue or PR was found for `get_block`
  accepting stale-but-known headers through the non-canonical `height_of_hash`
  path.
- `kyoto.tx_can_broadcast` is related only to
  https://github.com/2140-dev/kyoto/pull/556. That PR concerns transaction
  broadcast timeout behavior, not compact-filter or downloaded-block
  validation.

### Neutrino

- `NEUTRINO-BLOCKER-005` and
  https://github.com/lightninglabs/neutrino/pull/334: tried as a local
  replacement. It builds, but the black-box run still disconnects after the
  first 2000-header page, so it does not fix the current failure.
- `NEUTRINO-BLOCKER-005` and
  https://github.com/lightninglabs/neutrino/pull/282: too old to try as a
  drop-in fix. The adapter build fails against its older `ChainService` API.
- `NEUTRINO-SHOULD-003` and
  https://github.com/lightninglabs/neutrino/issues/349: directly related to
  bad-peer classification. The issue says compact-filter sync can ban peers
  for timeout/disconnect/follow-up query failure even when invalid
  compact-filter data was not proven.
- `NEUTRINO-SHOULD-003` and
  https://github.com/lightninglabs/neutrino/issues/338: related BIP157
  request-formation/reconnect-loop issue. It points at `getcfilters` and
  `getcfheaders` requests whose `StopHash` may not be known to the queried
  peer.
- `NEUTRINO-SHOULD-003` and
  https://github.com/lightninglabs/neutrino/issues/218: related
  persistence/recovery issue. It describes a node stuck after all peers
  previously advertised an invalid filter-header chain and a later peer exposes
  an invalid filter.
- `NEUTRINO-SHOULD-003` and
  https://github.com/lightninglabs/neutrino/pull/345: adjacent PR. It verifies
  imported compact filters against locally stored filter headers before
  persistence, but does not fix block-derived BIP158 exactness or the current
  P2P header-sync failure.
- `NEUTRINO-MUST-001` and `NEUTRINO-SHOULD-002`: no open Neutrino issue or PR
  was found specifically for `VerifyBasicBlockFilter` skipping coinbase outputs
  or unsupported/non-witness prevout-script derivation.
- Ban-state observability and recovery are adjacent to
  https://github.com/lightninglabs/neutrino/issues/253 because it asks for an
  unban API when bans are unfair. It is not a compact-filter verifier issue.
- https://github.com/lightninglabs/neutrino/pull/348 was reviewed and excluded
  from the finding set. It is about recognizing Tor v3 addresses in banman.

### Nakamoto

- `NAKAMOTO-BLOCKER-001` and https://github.com/cloudhead/nakamoto/pull/79:
  too old to try as a drop-in fix. It predates the current adapter package
  layout and does not provide the expected `nakamoto-net` package shape.
- https://github.com/cloudhead/nakamoto/pull/160 and
  https://github.com/cloudhead/nakamoto/pull/161 are potentially relevant
  modernization work, but neither was identified as a current compact-filter
  conformance fix.

### Wasabi

- `WASABI-MUST-001`, `WASABI-SHOULD-002`, and
  https://github.com/WalletWasabi/WalletWasabi/pull/14546: this PR is
  vendored locally as `0001-p2p-compact-filter-provider.patch`. The additional
  local patch `0002-anchor-height-one-to-genesis-filter-header.patch` fixes a
  BIP157 height-one bug still present in the PR.
- `WASABI-MUST-003` and
  https://github.com/WalletWasabi/WalletWasabi/pull/14025: adjacent
  block-download work. It does not address the invalid downloaded block or
  compact-filter validation failures in the strict adapter run.

## Historical Upstream Cross-Checks

Closed issues and PRs are useful for cross-project testing even when they no
longer describe a current bug in the project where they were filed. The
scenario backlog in `PROJECT_PLAN.md` records the concrete tests to add.

- Filter-header/cfilter disagreement: Neutrino issue
  https://github.com/lightninglabs/neutrino/issues/3 and PRs
  https://github.com/lightninglabs/neutrino/pull/130,
  https://github.com/lightninglabs/neutrino/pull/140, and
  https://github.com/lightninglabs/neutrino/pull/215 cover malicious or
  inconsistent compact-filter data. Keep these as black-box scenarios for
  every adapter.
- Exact BIP158 element rules: Neutrino PRs
  https://github.com/lightninglabs/neutrino/pull/55 and
  https://github.com/lightninglabs/neutrino/pull/84, plus issue
  https://github.com/lightninglabs/neutrino/issues/56, document full-script
  matching, OP_RETURN exclusion, and empty-filter serialization.
- False bans during ordinary chain progress: Kyoto issue
  https://github.com/2140-dev/kyoto/issues/558 and PR
  https://github.com/2140-dev/kyoto/pull/559 cover legitimate peers being
  banned when new block announcements race with in-flight compact-filter
  requests. This complements
  https://github.com/lightninglabs/neutrino/issues/349.
- Sync interruption and long cfheader batches: Neutrino issue
  https://github.com/lightninglabs/neutrino/issues/8 and PR
  https://github.com/lightninglabs/neutrino/pull/360 cover restart after
  interrupted cfheaders and long batch timeout behavior. Kyoto PRs
  https://github.com/2140-dev/kyoto/pull/391 and
  https://github.com/2140-dev/kyoto/pull/506 add filter-message timeout
  behavior.
- Header-validation edge cases: Neutrino PR
  https://github.com/lightninglabs/neutrino/pull/344 and Kyoto PRs
  https://github.com/2140-dev/kyoto/pull/109,
  https://github.com/2140-dev/kyoto/pull/273, and
  https://github.com/2140-dev/kyoto/pull/282 show past Median-Time-Past and
  header-context pitfalls.
- Witness block and prevout verification: Neutrino PR
  https://github.com/lightninglabs/neutrino/pull/215 and issue
  https://github.com/lightninglabs/neutrino/issues/257 relate to deriving spent
  scripts from block data. Kyoto PRs
  https://github.com/2140-dev/kyoto/pull/548 and
  https://github.com/2140-dev/kyoto/pull/549 cover optional witness-block
  requests.

## Current Suite Coverage Notes

The catalog intentionally includes all scenarios found in the current Kyoto and
Neutrino tests that can be represented by a black-box P2P harness, then adds
BIP157/BIP158 conformance scenarios for:

- BIP158 filter element rules, including coinbase, prevout scripts, OP_RETURN,
  and zero-element serialization.
- BIP157 peer disagreement with one honest peer and one lying peer.
- Direct bad cfilter handling.
- Self-consistent eclipse limitations.
- Temporary network delay during filter-header sync and block download.

Unimplemented catalog entries are written as `skipped` in every harness report
so coverage gaps remain visible instead of being silently treated as success.
