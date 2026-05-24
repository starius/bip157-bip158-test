# Address And Overlay Environments

The harness treats the address environment as an explicit matrix dimension.
Use `--environment` with `bip157-harness` and suffix matrix inputs as
`implementation@environment=report-dir`.

## Clear TCP

- `ipv4` binds peerlab servers to separate `127.27.0.N` loopback addresses.
  This gives adversarial-peer tests separate IP identities instead of only
  separate ports.
- `ipv6` binds peerlab servers to `::1`. Ordinary hosts expose only one IPv6
  loopback identity, so distinct IPv6 identity checks are reported as
  unsupported until a lab creates additional IPv6 identities.

## Overlay Labs

Overlay environments are planned but not yet active in the harness runtime.
The harness reports `env.<name>.full_matrix` as unsupported and skips adapter
scenarios instead of treating missing overlay setup as a BIP157/BIP158 failure.

Prepare deterministic lab skeletons with:

```sh
go run ./cmd/bip157-envlab --environment tor-v3 --peers 2 --out "$TMPDIR/tor-lab"
go run ./cmd/bip157-envlab --environment i2p --peers 2 --out "$TMPDIR/i2p-lab"
go run ./cmd/bip157-envlab --environment cjdns --peers 2 --out "$TMPDIR/cjdns-lab"
```

The Nix shell provides Tor, pinned Chutney source, Java I2P, i2pd, cjdns,
OpenJDK, and iproute2. The next implementation step is a privileged runner
that starts those labs and maps peerlab listeners to overlay identities.
