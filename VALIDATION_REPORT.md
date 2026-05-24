# Validation Report

Date: 2026-05-24

## Build and Unit Tests

The following checks passed in the reproducible Nix shell:

```sh
go test ./...
(cd adapters/neutrino && go test ./...)
(cd adapters/kyoto && cargo test)
(cd adapters/nakamoto && cargo test)
dotnet test adapters/wasabi.Tests/WasabiAdapter.Tests.csproj
```

The adapter binaries also built under the same shell. Kyoto and Nakamoto were
built with `cargo build --release`; the Wasabi adapter was built with the
local patch stack in `nix/patches/wasabi`.

## Matrix

The generated implementation matrix is saved in `IMPLEMENTATION_MATRIX.md`.
The catalog contains 88 scenarios. The active adapter subset now covers normal
sync, wallet receive/spend detection, long-chain checkpoint boundaries,
temporary outage recovery, explicit-peer mode, bad `cfheaders`, bad
`cfcheckpt`, bad `cfilter`, wrong filter type, unresponsive peers, invalid
downloaded blocks, and the currently imported Kyoto/Neutrino baseline rows.

| Implementation | Overall | Pass | Fail | Skipped | Notes |
| --- | --- | ---: | ---: | ---: | --- |
| no-adapter | green | 11 | 0 | 77 | Internal BIP158 vectors only. |
| fake adapter | green | 70 | 0 | 18 | Harness self-test target passed every active adapter scenario. |
| Kyoto adapter | red | 24 | 46 | 18 | Normal sync and outage recovery pass; adversarial peer handling and invalid downloaded blocks fail. |
| Wasabi adapter | red | 24 | 46 | 18 | Patched P2P client reaches the tip and scans filters, but fails adversarial peer handling and invalid downloaded blocks. |
| Neutrino adapter | red | 12 | 53 | 23 | Still disconnects after the first 2000-header page from peerlab in most P2P rows. |
| Nakamoto adapter | red | 11 | 54 | 23 | Connects to peerlab, but does not reach the fixture tip and often requests the genesis filter. |

## Failure Classification

No active scenario is currently classified as a bad test. The fake adapter
passes every active adapter row, and at least Kyoto or Wasabi passes the normal
long-chain, checkpoint, outage, and wallet-match rows that Neutrino and
Nakamoto fail. That makes the current Neutrino/Nakamoto failures real
adapter-or-library compatibility problems, not proof that the scenarios are
malformed.

Known adapter or observability issues:

- Kyoto returns `/list-peers` 503 in 25 failing rows after bad peer data. The
  transcript proves the bad data was served, but the adapter loses the ability
  to report peer state. Those rows remain failed, but their root cause is
  classified as adapter observability until isolated further.
- Neutrino's and Nakamoto's broad P2P failures are classified as unresolved
  adapter/library compatibility. They fail before most adversarial compact
  filter checks become meaningful.
- A Wasabi adapter peer-address mapping issue was fixed before this run; no
  remaining Wasabi failure is currently classified as adapter-only.

Known library or upstream-code issues:

- The Wasabi P2P compact-filter PR needed a local BIP157 fix:
  `0002-anchor-height-one-to-genesis-filter-header.patch`. Without it, a
  `cfheaders` range starting at height 1 is checked against zero instead of
  the genesis filter header.
- The patched Wasabi P2P client does not punish or otherwise expose rejection
  for corrupt `cfcheckpt`, corrupt `cfheaders`, bad `cfilter`, wrong filter
  type, or invalid downloaded block cases.
- Kyoto does not expose durable bad-peer punishment for the adversarial
  compact-filter rows. Some failures are obscured by the adapter 503 issue, but
  the rows that do return peer state still show "not punished" outcomes.
- Neutrino and Nakamoto still need focused protocol debugging before their
  failures can be assigned cleanly to the adapter or library.

## Implementation Notes

Kyoto and Wasabi both pass the main honest and recovery paths:

- honest wallet receive/spend
- long chain to height 2005
- compact-filter checkpoint boundaries
- large compact-filter batch progress
- best-block and block-hash API checks
- temporary `cfheaders` and block-download delay recovery
- explicit-peer/no-discovery mode

Kyoto and Wasabi both fail the active adversarial rows for corrupt or
conflicting compact-filter data. Wasabi reports cleaner "not punished"
evidence. Kyoto often reports `/list-peers` 503 after the bad transcript,
which needs adapter-level debugging before every Kyoto row can be classified
precisely.

Neutrino still fails the ordinary long-chain P2P path:

```text
getheaders -> headers(2000) -> disconnect EOF
```

Open PR `lightninglabs/neutrino#334` built but did not change this result.
Open PR `lightninglabs/neutrino#282` is too old for the current adapter API and
does not build as a drop-in replacement. PR `#345` is adjacent import
verification work, not a fix for the current P2P header-sync failure. PR
`#348` is unrelated Tor v3 banman work.

Nakamoto starts and handshakes, but does not reach height 2005. The transcript
often shows `getcfilters` for regtest genesis instead of long-chain header
progress. Open PR `cloudhead/nakamoto#79` is too old for the current adapter
package layout and was not a usable drop-in fix.

Wasabi is now a first-class strict target through the local adapter. Open PR
`WalletWasabi/WalletWasabi#14546` is vendored as
`0001-p2p-compact-filter-provider.patch`; the local
`0002-anchor-height-one-to-genesis-filter-header.patch` fixes a BIP157
height-one filter-header bug still present in that PR. Open PR `#14025` is
parallel block-download work and did not address the compact-filter validation
failures.

## Remaining Skipped Rows

The remaining 18 fake-adapter skipped rows are deliberate catalog entries that
need new harness machinery:

- one-block and two-block reorgs
- restart/persistence/import scenarios
- Neutrino initial interval permutations
- randomized block/filter generation
- self-consistent eclipse reporting
- transaction broadcast

These are not scored as pass. They remain visible in every run so the suite
does not silently lose coverage.
