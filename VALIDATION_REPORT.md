# Validation Report

Date: 2026-05-25

## Build and Unit Tests

The following checks passed in the reproducible Nix shell:

```sh
go test ./...
(cd adapters/neutrino && go test ./...)
(cd adapters/kyoto && cargo fmt --check && cargo test)
(cd adapters/nakamoto && cargo fmt --check && cargo test)
dotnet test adapters/wasabi.Tests/WasabiAdapter.Tests.csproj
```

The adapter binaries also built under the same shell. Kyoto and Nakamoto were
built with `cargo build --release`; the Wasabi adapter was built with the
local patch stack in `nix/patches/wasabi`.

## Matrix

The generated implementation matrix is saved in `IMPLEMENTATION_MATRIX.md`.
The catalog contains 96 scenarios. The active adapter subset covers normal
sync, wallet receive/spend detection, long-chain checkpoint boundaries,
temporary outage recovery, explicit-peer mode, bad `cfheaders`, bad
`cfcheckpt`, bad `cfilter`, wrong filter type, unresponsive peers, invalid
downloaded blocks, environment identity rows, and the imported Kyoto/Neutrino
baseline rows.

| Run | Overall | Pass | Fail | Unsupported | Skipped | Notes |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| `no-adapter@ipv4` | green | 12 | 0 | 0 | 84 | Internal vectors and IPv4 identity row. |
| `no-adapter@ipv6` | green | 12 | 0 | 0 | 84 | Internal vectors and IPv6 identity row. |
| `no-adapter@tor-v3` | green | 12 | 0 | 0 | 84 | Internal vectors and Tor identity row. |
| `no-adapter@i2p` | green | 11 | 0 | 1 | 84 | I2P runtime is still manifest-only. |
| `no-adapter@cjdns` | green | 11 | 0 | 1 | 84 | cjdns runtime is still manifest-only. |
| `fake@ipv4` | green | 72 | 0 | 0 | 24 | Harness self-test over distinct IPv4 peers. |
| `fake@ipv6` | green | 72 | 0 | 0 | 24 | Full run over distinct Linux ULA IPv6 peers. |
| `fake@tor-v3` | green | 72 | 0 | 0 | 24 | Full run through Chutney Tor v3 onion services. |
| `fake@i2p` | green | 11 | 0 | 2 | 83 | I2P adapter matrix is not active yet. |
| `fake@cjdns` | green | 11 | 0 | 2 | 83 | cjdns adapter matrix is not active yet. |
| `kyoto@ipv4` | red | 26 | 46 | 0 | 24 | Normal sync and outage recovery pass; adversarial rows fail. |
| `kyoto@ipv6` | green | 11 | 0 | 2 | 83 | Capability skip, not an IPv6 conformance pass. |
| `kyoto@tor-v3` | green | 11 | 0 | 2 | 83 | Capability skip, not a Tor conformance pass. |
| `nakamoto@ipv4` | red | 13 | 54 | 0 | 29 | Does not reach the long-chain fixture tip. |
| `nakamoto@ipv6` | green | 11 | 0 | 2 | 83 | Capability skip, not an IPv6 conformance pass. |
| `nakamoto@tor-v3` | green | 11 | 0 | 2 | 83 | Capability skip, not a Tor conformance pass. |
| `neutrino@ipv4` | red | 14 | 53 | 0 | 29 | Disconnects after the first 2000-header page in most rows. |
| `neutrino@ipv6` | green | 11 | 0 | 2 | 83 | Capability skip, not an IPv6 conformance pass. |
| `neutrino@tor-v3` | green | 11 | 0 | 2 | 83 | Capability skip, not a Tor conformance pass. |
| `wasabi@ipv4` | red | 31 | 41 | 0 | 24 | Syncs and scans, but still fails many adversarial rows. |
| `wasabi@ipv6` | green | 11 | 0 | 2 | 83 | Capability skip, not an IPv6 conformance pass. |
| `wasabi@tor-v3` | green | 11 | 0 | 2 | 83 | Capability skip, not a Tor conformance pass. |

All real-adapter I2P and cjdns rows are also capability skips with 11 pass,
0 fail, 2 unsupported, and 83 skipped.

## Environment Coverage

IPv4 is the baseline environment. Peerlab binds separate peers to
`127.27.0.N`, so bad-peer scenarios can distinguish peers by IP identity.

IPv6 now has an active distinct-identity lab. The `linux-iproute` allocator
assigns deterministic `fd7a:b157:b158::/64` loopback addresses, keeps leases
until harness shutdown, and removes them during cleanup. The fake adapter
passed the full IPv6 matrix. Real adapters remain disabled for IPv6 until they
prove bracketed IPv6 dialing in connect-only regtest mode.

Tor v3 now has an active Chutney runtime. The harness starts a private
`hs-v3-min` network, creates ephemeral v3 onion services for peerlab listeners,
advertises onion peer addresses, and passes the Chutney SOCKS endpoint to the
adapter. The fake adapter passed the full Tor matrix. Real adapters remain
disabled for Tor until they use the supplied SOCKS endpoint or native onion
transport.

I2P and cjdns still have manifest scaffolding only. Their rows are explicit
unsupported capability results, not conformance passes.

## Failure Classification

No active scenario is currently classified as a bad test. The fake adapter
passes every active adapter row over IPv4, IPv6, and Tor v3, and at least Kyoto
or Wasabi passes the normal long-chain, checkpoint, outage, and wallet-match
rows that Neutrino and Nakamoto fail. That makes the current IPv4
Neutrino/Nakamoto failures real adapter-or-library compatibility problems, not
proof that the scenarios are malformed.

Known adapter or observability issues:

- Kyoto returns `/list-peers` 503 in many failing rows after bad peer data. The
  transcript proves the bad data was served, but the adapter loses the ability
  to report peer state. Those rows remain failed, but their root cause is
  classified as adapter observability until isolated further.
- Neutrino's and Nakamoto's broad P2P failures are classified as unresolved
  adapter/library compatibility. They fail before most adversarial compact
  filter checks become meaningful.
- Real-adapter IPv6 and Tor rows are capability skips. They should stay that
  way until each adapter has real dialer support for the configured transport.

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

Kyoto and Wasabi both pass the main honest and recovery paths over IPv4:

- honest wallet receive/spend
- long chain to height 2005
- compact-filter checkpoint boundaries
- large compact-filter batch progress
- best-block and block-hash API checks
- temporary `cfheaders` and block-download delay recovery
- explicit-peer/no-discovery mode

Kyoto and Wasabi both fail active adversarial rows for corrupt or conflicting
compact-filter data. Wasabi reports cleaner "not punished" evidence. Kyoto
often reports `/list-peers` 503 after the bad transcript, which needs
adapter-level debugging before every Kyoto row can be classified precisely.

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

Wasabi is a strict target through the local adapter. Open PR
`WalletWasabi/WalletWasabi#14546` is vendored as
`0001-p2p-compact-filter-provider.patch`; the local
`0002-anchor-height-one-to-genesis-filter-header.patch` fixes a BIP157
height-one filter-header bug still present in that PR. Open PR `#14025` is
parallel block-download work and did not address the compact-filter validation
failures.

## Remaining Skipped Rows

Each single-environment fake-adapter run has 24 skipped rows. Six are expected
environment-selection skips: the four non-selected environment rows plus the
two non-selected identity rows. The other 18 are deliberate catalog entries
that need new harness machinery:

- one-block and two-block reorgs
- restart/persistence/import scenarios
- Neutrino initial interval permutations
- randomized block/filter generation
- self-consistent eclipse reporting
- transaction broadcast

These are not scored as pass. They remain visible in every run so the suite
does not silently lose coverage.
