# Adapter API and Build Guide

The suite treats every BIP157/BIP158 implementation as a black-box HTTP
adapter. The canonical schema is in `proto/bip157test.proto`; the current
transport is HTTP JSON with the same field names.

The harness now passes an explicit `environment` object to `/configure` and
annotates each peer with `address_type`, `transport`, optional `proxy_address`,
and a peer `identity`. Older adapters may ignore these fields. New adapters
should implement `/capabilities` so the harness can distinguish unsupported
address environments from BIP157/BIP158 failures.

## Required Endpoints

All endpoints use `POST`.

| Endpoint | Purpose |
| --- | --- |
| `/configure` | Reset the adapter for one isolated regtest run and provide harness-controlled peers. |
| `/capabilities` | Optionally report supported address environments. |
| `/start` | Start networking and syncing. |
| `/stop` | Stop networking and release resources. |
| `/watch-script` | Track a scriptPubKey from a start height. |
| `/matches` | Return wallet-relevant transactions found for the script. |
| `/best-block` | Return the adapter's best known block hash and height. |
| `/block-hash` | Return the current-chain block hash at a height. |
| `/list-peers` | Return connected/banned/error state for harness peers. |
| `/health` | Return process liveness and coarse state. |

Adapters should listen on `127.0.0.1:0` by default and print
`listening=http://host:port` on startup. The harness can then run:

```sh
go run ./cmd/bip157-harness --adapter-url "$ADAPTER_URL" --data-dir "$TMPDIR/adapter"
go run ./cmd/bip157-harness --environment ipv6 --adapter-url "$ADAPTER_URL" --data-dir "$TMPDIR/adapter"
go run ./cmd/bip157-harness --environment tor-v3 --tor-lab chutney --adapter-url "$ADAPTER_URL" --data-dir "$TMPDIR/adapter"
```

## Writing a New Adapter

1. Parse `/configure`, reject non-regtest networks, and connect only to the
   supplied peers unless `allow_discovery` is true.
2. Make `/start` non-blocking. Sync should happen in implementation-owned
   goroutines/tasks.
3. Implement `/watch-script` for arbitrary scriptPubKeys when possible. If the
   library only supports address watches, document the supported script forms.
4. Report both output matches and spend matches. A spend match means a
   transaction spends a previously watched output.
5. Implement `/list-peers` from the implementation's real peer state. A peer
   does not have to be permanently banned to pass every SHOULD scenario, but it
   must expose enough state for the harness to distinguish accepted, rejected,
   disconnected, and banned peers.
6. Keep adapter state isolated by `data_dir`. The harness will reuse the same
   adapter process across scenarios and call `/configure` between them.
7. Use `PeerConfig.ProxyAddress` or `Environment.ProxyAddress` for Tor or I2P
   transports. The harness treats an onion or I2P address as unsupported until
   the adapter explicitly advertises that environment.
8. Implement `/capabilities` when the adapter supports more than clear IPv4.
   Missing capabilities default to IPv4-only support.

## Included Adapters

### Fake

```sh
go run ./cmd/fake-adapter --listen 127.0.0.1:0
```

This adapter is a harness self-test target backed by the deterministic fixture.
It is not a BIP157 implementation.

### Neutrino

```sh
cd adapters/neutrino
go test ./...
go build -o neutrino-adapter .
./neutrino-adapter --listen 127.0.0.1:0
```

The Neutrino adapter pins
[`lightninglabs/neutrino`](https://github.com/lightninglabs/neutrino) through
`adapters/neutrino/go.mod`. It wraps `ChainService`, watches P2WPKH scripts
through Neutrino's rescan API, and records output/spend matches from filtered
blocks.

### Kyoto

```sh
cd adapters/kyoto
cargo test
cargo build --release
./target/release/kyoto-adapter --listen 127.0.0.1:0
```

The Kyoto adapter pins
[`2140-dev/kyoto`](https://github.com/2140-dev/kyoto) through
`adapters/kyoto/Cargo.toml`. It consumes `IndexedFilter` events, requests
matching blocks through Kyoto, and records output/spend matches from those
blocks.

### Nakamoto

```sh
cd adapters/nakamoto
cargo test
cargo build --release
./target/release/nakamoto-adapter --listen 127.0.0.1:0
```

The Nakamoto adapter pins
[`cloudhead/nakamoto`](https://github.com/cloudhead/nakamoto) through
`adapters/nakamoto/Cargo.toml`. It runs Nakamoto in regtest mode, sets the
harness-supplied peers as explicit `connect` peers, listens only on localhost,
and records output/spend matches from blocks Nakamoto downloads after
compact-filter matches.

Nakamoto does not currently expose a durable ban list through the public handle.
The adapter therefore reports a peer as non-banned but disconnected with an
error when the peer is no longer connected. The harness treats that as enough
evidence for SHOULD-level "reject or punish" cases only when the peerlab
transcript proves the bad BIP157 response was actually served.

### Wasabi

```sh
src=$(./adapters/wasabi/prepare-source.sh adapters/wasabi/.wasabi-src)
export WASABI_PATCHED_SOURCE="$src"
dotnet test adapters/wasabi.Tests/WasabiAdapter.Tests.csproj
dotnet build adapters/wasabi/WasabiAdapter.csproj
dotnet adapters/wasabi/bin/Debug/net10.0/wasabi-adapter.dll --listen 127.0.0.1:0
```

The Wasabi adapter pins
[`WalletWasabi/WalletWasabi`](https://github.com/WalletWasabi/WalletWasabi)
through the Nix flake. Wasabi's current standard-filter synchronization path
uses Bitcoin RPC filter calls, so the suite applies local patches under
`nix/patches/wasabi` before building:

- `0001-p2p-compact-filter-provider.patch` is a squashed local copy of the
  Wasabi P2P compact-filter PR.
- `0002-anchor-height-one-to-genesis-filter-header.patch` fixes height-one
  filter-header validation so BIP157 `cfheaders` ranges starting after genesis
  are anchored to the genesis filter header.

The adapter is a library-level wrapper around the patched Wasabi P2P code. It
constructs a harness-controlled regtest peer set, attaches Wasabi's header and
compact-filter behaviors, scans Wasabi's filter store for watched scripts, and
downloads matching blocks over P2P through Wasabi's block provider.
