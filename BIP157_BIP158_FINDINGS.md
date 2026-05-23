# BIP157/BIP158 Findings

This file records implementation mismatches and limitations found while
building the conformance suite. `MUST` failures should score red. Missing or
weak `SHOULD` behavior should score orange.

## Kyoto

| ID | Severity | Finding | Evidence | Expected Test Coverage |
| --- | --- | --- | --- | --- |
| KYOTO-SHOULD-001 | SHOULD | A self-consistent malicious peer set can supply matching bad filter headers and bad filters without a block-derived recomputation check detecting the lie. This is a trust-limit case, but it must be reported as such rather than described as full BIP158 validation. | Filter events are accepted from received cfilters after header-chain checks; the public API exposes matched filters and block fetches, not a full local filter derivation path. See `/home/user/kyoto/src/chain/chain.rs:334` and `/home/user/kyoto/src/client.rs:115`. | `bip157.self_consistent_eclipse` |
| KYOTO-SHOULD-002 | SHOULD | A single lying peer can cause a sync or match scenario to fail by serving conflicting compact-filter data; Kyoto exposes disconnect-style peer churn but not a clear persistent ban result to the adapter API. | Block requests are sent to a random peer from the queue (`/home/user/kyoto/src/node.rs:307`). Peer state exposed by `peer_info` lists current connections, not ban status (`/home/user/kyoto/src/client.rs:179`). | `bip157.conflict_one_honest_one_liar`, `bip157.direct_bad_cfilter_ban` |
| KYOTO-MUST-003 | MUST-risk | `get_block` accepts any known header hash, while the user-facing contract around related helpers is current-chain oriented. Kyoto has added a canonical-only helper for the public `height_of_hash` path, but `get_block` and block handling still use the broader known-header lookup. | `height_of_hash_canonical_only` exists at `/home/user/kyoto/src/chain/graph.rs:344`; `get_block` still checks `height_of_hash` at `/home/user/kyoto/src/node.rs:211`; received blocks are also indexed with `height_of_hash` at `/home/user/kyoto/src/node.rs:541`. | `kyoto.stop_reorg_start_on_orphan`, plus a new stale-header block-request test |
| KYOTO-SHOULD-004 | SHOULD | Peer disagreement resolution is not yet exposed as a strong black-box outcome. A conformant client should detect disagreement across peers and recover via an honest peer or a block-derived filter check. | The adapter can observe current peers and best block, but Kyoto does not expose a direct "peer banned for bad cfheaders/cfilter" query. | `bip157.conflict_one_honest_one_liar` |

## Neutrino

| ID | Severity | Finding | Evidence | Expected Test Coverage |
| --- | --- | --- | --- | --- |
| NEUTRINO-MUST-001 | MUST | `VerifyBasicBlockFilter` skips the coinbase transaction even though BIP158 basic filters include coinbase output scripts except unspendable outputs. | The verifier skips index 0 at `/home/user/neutrino/verification.go:22`. | `bip158.coinbase_output_included` |
| NEUTRINO-SHOULD-002 | SHOULD | Filter verification cannot derive every input prevout script from the spending transaction. Non-witness and unsupported witness/script forms are skipped, so conflict resolution can fall back to peer majority rather than exact filter derivation. | Unsupported or non-witness inputs are skipped at `/home/user/neutrino/verification.go:92`; the fallback majority branch is documented at `/home/user/neutrino/blockmanager.go:1793`. | `bip158.prevout_legacy_included`, future taproot and script-coverage cases |
| NEUTRINO-SHOULD-003 | SHOULD | Bad-filter peer resolution is meaningful but not complete for all BIP158 element sets because it relies on partial verification plus OP_RETURN mismatch heuristics. | OP_RETURN mismatch counting and potential bans are at `/home/user/neutrino/blockmanager.go:1766`; unit coverage exists in `/home/user/neutrino/bamboozle_unit_test.go:657`. | `bip157.direct_bad_cfilter_ban`, `neutrino.resolve_filter_mismatch.*` |
| NEUTRINO-TEST-004 | Test gap | Neutrino has deeper internal tests than Kyoto, including real `rpctest` nodes and conflict-resolution unit tests, but those tests are implementation-specific and not a reusable BIP157/BIP158 conformance suite. | SimNet/rpctest setup appears in `/home/user/neutrino/sync_test.go:1088` and multi-peer setup at `/home/user/neutrino/sync_test.go:1465`. | The suite catalog imports all listed Neutrino baseline scenarios and adds stronger black-box adversarial scenarios. |

## Upstream Issue and PR Cross-References

Checked against open GitHub issues and PRs on 2026-05-23. These are
cross-references for future upstream reports. They are not substitutes for the
local evidence above.

### Kyoto

| Local Finding | Upstream Item | Relationship |
| --- | --- | --- |
| KYOTO-SHOULD-001, KYOTO-SHOULD-002, KYOTO-SHOULD-004 | https://github.com/2140-dev/kyoto/issues/561 | Partial overlap. The issue is about reorganization handling while filter data is in flight, including `UnknownFilterHash` behavior and peer banning/exhaustion. It is relevant when discussing reorg/filter race and peer-punishment tests, but it is not a direct report of the self-consistent eclipse case. |
| KYOTO-SHOULD-004 | https://github.com/2140-dev/kyoto/issues/538 | Adjacent API-observability issue. It asks for separate header, filter, and block-fetch process control, which would make conformance adapters and reports more precise. It is not itself a BIP157/BIP158 mismatch. |
| KYOTO-MUST-003 | none found | I did not find an open Kyoto issue or PR for `get_block` accepting stale-but-known headers through the non-canonical `height_of_hash` path. |
| All Kyoto findings | https://github.com/2140-dev/kyoto/pull/556 | Reviewed and excluded from the BIP157/BIP158 finding set. The PR concerns transaction broadcast timeout behavior, not compact filters or downloaded-block/filter verification. |

### Neutrino

| Local Finding | Upstream Item | Relationship |
| --- | --- | --- |
| NEUTRINO-SHOULD-003 | https://github.com/lightninglabs/neutrino/issues/349 | Directly related to bad-peer classification. The issue says compact-filter sync can ban peers for timeout/disconnect/follow-up query failure even when invalid compact-filter data was not proven. |
| NEUTRINO-SHOULD-003 | https://github.com/lightninglabs/neutrino/issues/338 | Related BIP157 request-formation/reconnect-loop issue. It points at `getcfilters`/`getcfheaders` requests whose `StopHash` may not be known to the queried peer, causing bitcoind disconnect loops during reorg conditions. |
| NEUTRINO-SHOULD-003 | https://github.com/lightninglabs/neutrino/issues/218 | Related persistence/recovery issue. It describes a node stuck after all peers previously advertised an invalid filter-header chain and a later peer exposes an invalid filter. |
| NEUTRINO-SHOULD-003 | https://github.com/lightninglabs/neutrino/pull/345 | Adjacent PR. It verifies imported compact filters against locally stored filter headers before persistence. That helps imported-filter integrity, but it does not fix block-derived BIP158 exactness for coinbase outputs or skipped prevout scripts. |
| NEUTRINO-MUST-001, NEUTRINO-SHOULD-002 | none found | I did not find an open Neutrino issue or PR specifically covering `VerifyBasicBlockFilter` skipping coinbase transaction outputs or skipping unsupported/non-witness prevout-script derivation. |
| Ban-state observability and recovery | https://github.com/lightninglabs/neutrino/issues/253 | Adjacent to false-ban tests because it asks for an unban API when bans are unfair. It is not a compact-filter verifier issue. |
| All Neutrino findings | https://github.com/lightninglabs/neutrino/pull/348 | Reviewed and excluded from the finding set. The PR is about recognizing Tor v3 addresses in banman; it is not about whether compact-filter sync should ban on unproven bad data. |

## Current Suite Coverage Notes

The catalog intentionally includes all scenarios found in the current Kyoto and
Neutrino tests, then adds BIP157/BIP158 conformance scenarios for:

- BIP158 filter element rules, including coinbase, prevout scripts, OP_RETURN,
  and zero-element serialization.
- BIP157 peer disagreement with one honest peer and one lying peer.
- Direct bad cfilter handling.
- Self-consistent eclipse limitations.
- Temporary network delay during filter-header sync and block download.

Unimplemented catalog entries are written as `skipped` in every harness report
so coverage gaps remain visible instead of being silently treated as success.
