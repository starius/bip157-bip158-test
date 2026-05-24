# Validation Report

Date: 2026-05-23

## Build and Unit Tests

The following checks passed in the reproducible Nix shell:

```sh
go test ./...
cd adapters/neutrino && go test ./...
cd adapters/kyoto && cargo test
```

Coverage includes the suite packages, long-chain fixture builder, peer
simulator, scoring, scenario catalog, fake adapter, Neutrino adapter helper
logic, and Kyoto adapter helper logic.

## Harness Matrix

The harness catalog currently contains 82 scenarios. The executable subset now
includes BIP158 vectors, long BIP157 header/filter sync, checkpoint boundaries,
adapter API checks, temporary delays, and several adversarial peer cases.

| Implementation | Overall | Notes |
| --- | --- | --- |
| fake adapter | green | Self-test target passed every executable scenario. |
| Kyoto adapter | red | Mandatory malformed `cfheaders` previous-header check failed. Several SHOULD peer-punishment checks also failed. |
| Neutrino adapter | red | Did not complete long-chain header sync with peerlab; every adapter scenario that needs tip sync failed before compact-filter messages were requested. |

## Fake Adapter

The fake adapter produced a green report. It is not a Bitcoin client; it exists
to prove the harness, scoring, and adapter API can produce a clean run.

Important passes:

- all BIP158 internal vectors
- honest wallet receive/spend
- long-chain tip sync
- adapter block-hash and peer API checks
- bad `cfcheckpt`, bad previous filter header, bad `cfilter`, wrong filter
  type, conflicting `cfheaders`, and unresponsive peer simulations
- temporary `cfheaders` and block-download delays

## Kyoto Adapter

Kyoto produced a red report.

Mandatory passes:

- BIP158 internal vectors
- honest wallet receive/spend
- long chain to height 2005
- large header/filter batch progress
- `cfheaders` checkpoint-boundary sync
- adapter best-block, block-hash, unknown-height, and peer API checks
- temporary `cfheaders` and block-download delay recovery
- multi-peer initial sync baseline
- one-shot and long rescan result baselines

Mandatory failure:

| Scenario | Result | Evidence |
| --- | --- | --- |
| `neutrino.blockmanager_invalid_interval.invalid_prev_header` / `bip157.bad_cfheaders_prev_header` | fail | Kyoto reached the target tip after peerlab served a corrupt `PrevFilterHeader`; no ban, disconnect, or adapter-visible error was observed. |

SHOULD failures:

| Scenario | Result | Evidence |
| --- | --- | --- |
| `bip157.bad_cfcheckpt_response` / `neutrino.cfcheckpt_sanity.case_1` | fail | Corrupt compact-filter checkpoint was not punished. |
| `bip157.conflict_one_honest_one_liar` / `neutrino.cfheaders_mismatch.case_1` | fail | Lying `cfheaders` peer was not punished. |
| `bip157.direct_bad_cfilter_ban` / `neutrino.detect_bad_peers.filter_hash_mismatch` | fail | Corrupt `cfilter` peer was not punished. |
| `bip157.wrong_filter_type_response` | fail | Wrong filter type response was not punished. |
| `neutrino.detect_bad_peers.unresponsive_peer` | fail | Peer-state query returned 503 during the stalled-peer scenario. |

Interpretation: Kyoto handles the ordinary long-chain and recovery paths, but
the current executable suite observes a mandatory invalid-filter-header
acceptance path and missing SHOULD-level adversarial peer handling.

## Neutrino Adapter

Neutrino produced a red report.

Internal BIP158 vectors passed because they are independent suite checks. The
adapter scenarios that need Neutrino to sync with peerlab failed before compact
filter messages were requested.

Representative failure:

```text
getheaders -> headers(2000) -> disconnect EOF
```

This pattern appeared in the honest run, long-chain run, checkpoint-boundary
run, outage runs, multi-peer initial sync, and all adversarial runs. Because the
bad `cfheaders`, `cfcheckpt`, and `cfilter` responses were never served, those
adversarial scenarios are now reported as failures or blocked by the same
header-sync obstacle rather than as successful peer punishment.

Mandatory failures include:

- `adapter.honest_wallet_receive_spend`
- `chain.long_checkpointed_header_sync`
- `bip157.large_batch_progress_timeout`
- `bip157.cfheaders_order_and_checkpoint_boundaries`
- `kyoto.various_client_methods`
- `network.outage_filter_headers`
- `network.outage_block_download`
- `neutrino.sync_without_headers_import.initial_sync`
- `neutrino.blockmanager_invalid_interval.invalid_prev_header`

Interpretation: this is either a peerlab/Neutrino interoperability gap around
header pagination or a Neutrino behavior difference from the existing SimNet
tests. It is still a valid red result for this black-box suite because the
adapter did not reach the required best block or wallet-match API contract.

## Remaining Gaps

- Reorg and persistence scenarios remain cataloged but not executable.
- The self-consistent eclipse scenario remains cataloged because BIP157 cannot
  guarantee detection when every peer serves a mutually consistent false filter
  chain.
- Several Neutrino baseline permutations remain cataloged only; the current
  first-page header-sync failure must be resolved before they can produce useful
  compact-filter evidence.
- Optional network emulation with packet loss, reordering, duplication, and
  privileged `netem` mode is still pending.
