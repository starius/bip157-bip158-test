# Address And Overlay Environments

The harness treats the address environment as an explicit matrix dimension.
Use `--environment` with `bip157-harness` and suffix matrix inputs as
`implementation@environment=report-dir`.

## Clear TCP

- `ipv4` binds peerlab servers to separate `127.27.0.N` loopback addresses.
  This gives adversarial-peer tests separate IP identities instead of only
  separate ports.
- `ipv6` uses the address-lab selected by `--address-lab`. The
  non-privileged `loopback` lab binds every peer to `::1` and reports collapsed
  peer identities. The Linux `iproute2` lab assigns deterministic
  `fd7a:b157:b158::<peer>/128` addresses to loopback, binds peerlab listeners
  to those addresses, and removes them during harness cleanup.

Use strict identity mode when a scenario cannot be interpreted with collapsed
peer identities:

```sh
go run ./cmd/bip157-harness \
  --environment ipv6 \
  --address-lab linux-iproute \
  --require-distinct-identities \
  --adapter-url "$ADAPTER_URL" \
  --data-dir "$TMPDIR/adapter"
```

The Linux allocator needs permission to add and remove loopback IPv6
addresses, for example through `CAP_NET_ADMIN` or an equivalent privileged
runner. Real adapters are only scored on IPv6 when they advertise IPv6 support;
otherwise their rows remain explicit capability skips.

## Overlay Labs

Tor v3 has an active Chutney-backed runtime. The harness starts a private
`hs-v3-min` network, creates one ephemeral v3 onion service per peerlab
listener, advertises `<service>.onion:<port>` as the peer address, and passes
the Chutney SOCKS endpoint through `Environment.ProxyAddress` and
`PeerConfig.ProxyAddress`.

Run Tor v3 scenarios with:

```sh
go run ./cmd/bip157-harness \
  --environment tor-v3 \
  --tor-lab chutney \
  --adapter-url "$ADAPTER_URL" \
  --data-dir "$TMPDIR/adapter"
```

Adapters must use the supplied SOCKS endpoint or their native onion transport
before they can claim Tor support. The fake adapter already passes the full Tor
v3 matrix and proves the harness path is active.

I2P and cjdns are still manifest-only. The harness reports their full-matrix
rows as unsupported and skips adapter scenarios instead of treating missing
overlay setup as a BIP157/BIP158 failure.

Prepare deterministic lab skeletons with:

```sh
go run ./cmd/bip157-envlab --environment tor-v3 --peers 2 --out "$TMPDIR/tor-lab"
go run ./cmd/bip157-envlab --environment i2p --peers 2 --out "$TMPDIR/i2p-lab"
go run ./cmd/bip157-envlab --environment cjdns --peers 2 --out "$TMPDIR/cjdns-lab"
```

The Nix shell provides Tor, pinned Chutney source, Java I2P, i2pd, cjdns,
OpenJDK, and iproute2. The remaining overlay implementation work is for I2P
and cjdns runtimes that start those labs and map peerlab listeners to overlay
identities.
