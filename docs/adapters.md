# Adapter API and Build Guide

The suite treats every BIP157/BIP158 implementation as a black-box HTTP
adapter. The canonical schema is in `proto/bip157test.proto`; the current
transport is HTTP JSON with the same field names.

## Required Endpoints

All endpoints use `POST`.

| Endpoint | Purpose |
| --- | --- |
| `/configure` | Reset the adapter for one isolated regtest run and provide harness-controlled peers. |
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

The Neutrino adapter expects the Neutrino checkout at `../neutrino` relative to
the suite root. It wraps `ChainService`, watches P2WPKH scripts through
Neutrino's rescan API, and records output/spend matches from filtered blocks.

### Kyoto

```sh
cd adapters/kyoto
cargo test
cargo build --release
./target/release/kyoto-adapter --listen 127.0.0.1:0
```

The Kyoto adapter expects the Kyoto checkout at `../kyoto` relative to the suite
root. It consumes `IndexedFilter` events, requests matching blocks through
Kyoto, and records output/spend matches from those blocks.
