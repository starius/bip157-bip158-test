# Validation Report

Date: 2026-05-23

## Build and Unit Tests

The following checks passed in the reproducible Nix shell:

```sh
go test ./...
cd adapters/neutrino && go test ./...
cd adapters/kyoto && cargo test
cd adapters/nakamoto && cargo test
```

The shell is pinned by `flake.lock` and includes Go, Rust/Cargo, Bitcoin Core,
protobuf tooling, and .NET 10 for future Wasabi experiments.

## Matrix

The generated implementation matrix is saved in `IMPLEMENTATION_MATRIX.md`.
The catalog now contains 88 scenarios. The current executable subset contains
37 passing scenarios for the fake adapter and includes BIP158 vectors, long
BIP157 header/filter sync, temporary outage recovery, explicit-peer mode, and
adversarial `cfheaders`, `cfcheckpt`, `cfilter`, and downloaded-block cases.

| Implementation | Overall | Pass | Fail | Skipped | Notes |
| --- | --- | ---: | ---: | ---: | --- |
| no-adapter | green | 10 | 0 | 78 | Internal BIP158 vectors only. |
| fake adapter | green | 37 | 0 | 51 | Harness self-test target passed every executable scenario. |
| Kyoto adapter | red | 23 | 14 | 51 | Ordinary sync passes; adversarial validation and invalid-block checks fail. |
| Neutrino adapter | red | 10 | 22 | 56 | Still disconnects after the first 2000-header page from peerlab. |
| Nakamoto adapter | red | 10 | 22 | 56 | Adapter starts, but does not advance past peerlab genesis/filter interaction. |

## Kyoto

Kyoto passes the main honest and recovery paths:

- honest wallet receive/spend
- long chain to height 2005
- large compact-filter batch progress
- checkpoint-boundary sync
- best-block, block-hash, unknown-height, and peer API checks
- temporary `cfheaders` and block-download delay recovery
- explicit-peer/no-discovery mode

Kyoto fails two executable MUST rows:

- `blocks.invalid_downloaded_block_rejected`
- `neutrino.blockmanager_invalid_interval.invalid_prev_header`

Kyoto also fails the executable SHOULD adversarial rows for bad `cfcheckpt`,
bad `PrevFilterHeader`, empty `cfheaders`, conflicting `cfheaders`, corrupt or
malformed `cfilter`, wrong `cfilter` block hash, wrong filter type, and
unresponsive peer handling. The common pattern is that the adapter either
reaches the tip or cannot report a peer punishment/error after peerlab serves
the bad data.

## Neutrino

Neutrino passes only the implementation-independent BIP158 vectors in this
black-box run. Every adapter scenario that requires long-chain sync fails
before compact-filter messages are requested.

Representative peer transcript:

```text
getheaders -> headers(2000) -> disconnect EOF
```

This remains either a peerlab/Neutrino interoperability gap around header
pagination or a behavior difference from Neutrino's internal SimNet tests. It
is still a red black-box result because the adapter does not satisfy the
required best-block and wallet-match API contract.

## Nakamoto

The new Nakamoto adapter builds and its unit tests pass. In the black-box run,
Nakamoto connects to peerlab and exchanges version/verack/ping traffic, but it
does not reach the long-chain tip. Some scenarios show repeated or initial
`getcfilters` requests for the regtest genesis block, but no successful
long-chain header progress.

The current result should be treated as an adapter/peerlab compatibility
obstacle until isolated further. It is scored red because the strict adapter
contract requires the implementation to reach the harness tip and report wallet
matches from P2P data.

## Wasabi

Wasabi master remains outside the strict BIP157 P2P matrix because its current
standard-filter path uses Bitcoin RPC filter calls.

The evaluated P2P compact-filter PR branch is still experimental. It contains
useful validation logic for empty `cfheaders`, wrong ranges, previous-header
mismatches, malformed GCS filters, and filter-header mismatches, and those
ideas are now represented in the suite. It is not yet a strict adapter target:
the app startup path still constructs and monitors Bitcoin RPC, and the
regtest P2P helper hardcodes the default regtest peer port instead of accepting
harness-supplied peers.

## Remaining Gaps

- Reorg and persistence scenarios remain cataloged but not executable.
- The self-consistent eclipse scenario remains cataloged as a trust-limit case.
- Several Neutrino baseline permutations remain cataloged only.
- Nakamoto needs a focused adapter/peerlab compatibility investigation before
  its failures can be classified as implementation bugs.
- Wasabi needs either upstream explicit-peer regtest support or a smaller
  library-level adapter that bypasses the normal app startup path.
