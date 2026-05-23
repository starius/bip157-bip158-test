# Validation Report

Date: 2026-05-23

## Build and Unit Tests

The following checks passed in the reproducible Nix shell:

```sh
go test ./...
cd adapters/neutrino && go test ./...
cd adapters/kyoto && cargo test
```

Coverage at this point includes the suite packages, peer simulator, scoring,
scenario catalog, fake adapter, Neutrino adapter helper logic, and Kyoto adapter
helper logic.

## Harness Self-Test

The reference fake adapter produced a green executable-subset report:

| Scenario | Result |
| --- | --- |
| Honest wallet receive and spend | pass |
| One honest peer and one lying cfheaders peer | pass |
| Direct corrupt cfilter peer | pass |
| Temporary block download delay | pass |
| Temporary cfheaders delay | pass |

Skipped catalog entries remain visible in the generated report. Skips do not
improve the score.

## Kyoto Adapter Run

Kyoto produced an orange report:

| Scenario | Level | Result | Notes |
| --- | --- | --- | --- |
| Honest wallet receive and spend | MUST | pass | Kyoto reported both the watched output and spend. |
| Temporary cfheaders delay | MUST | pass | Kyoto reached the fixture tip after a one-shot cfheaders delay. |
| Temporary block download delay | MUST | pass | Kyoto reported both matches after a one-shot block delay. |
| One honest peer and one lying cfheaders peer | SHOULD | fail | The adapter could not query peer state after the adversarial run; `/list-peers` returned 503. |
| Direct corrupt cfilter peer | SHOULD | fail | Same peer-state query failure after the adversarial run. |

Interpretation: the current executable subset finds no Kyoto MUST failure, but
it does not observe the requested SHOULD-level peer punishment behavior.

## Neutrino Adapter Run

Neutrino produced a red report:

| Scenario | Level | Result | Notes |
| --- | --- | --- | --- |
| Honest wallet receive and spend | MUST | fail | The adapter did not reach the compact-filter-ready tip. |
| Temporary cfheaders delay | MUST | fail | Same tip timeout. |
| Temporary block download delay | MUST | fail | Same tip timeout. |
| One honest peer and one lying cfheaders peer | SHOULD | fail | Same tip timeout before disagreement resolution could be observed. |
| Direct corrupt cfilter peer | SHOULD | fail | Same tip timeout before corrupt-filter punishment could be observed. |

The peer transcript for the honest run shows Neutrino successfully handshaking,
requesting headers, receiving two headers, then requesting headers again and
receiving zero. It never requested compact filter headers from the short
three-block fixture before the harness timeout. This may be a Neutrino short
regtest-chain assumption, a simulator compatibility gap, or both; it is still a
valid red result for the current black-box suite because the adapter did not
complete the required API contract.

## Current Limitations

- The scenario catalog contains every Kyoto and Neutrino baseline scenario found
  during analysis, but many baseline scenarios are still catalog-only and
  reported as skipped.
- The executable peerlab chain is intentionally tiny. Neutrino's existing tests
  use much deeper SimNet chains, including 800-block and multi-peer setups, so a
  future suite iteration should add a long-chain fixture mode for checkpoint
  interval and cfheaders synchronization behavior.
- Taproot prevout coverage is cataloged but not implemented yet.
- The current adapters expose the best available peer state through public APIs.
  Kyoto does not expose persistent ban state, and the Neutrino adapter cannot
  query its ban store directly.
