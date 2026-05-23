# Baseline Coverage Policy

The conformance suite must not be weaker than the implementation-specific
tests that already exist in Kyoto and Neutrino.

The scenario catalog therefore includes:

- 8 Kyoto baseline scenarios matching the integration tests in
  `/home/user/kyoto/tests/core.rs`.
- 53 Neutrino baseline scenarios matching the sync, blockmanager, bamboozle,
  and filter-verification cases in `/home/user/neutrino`.
- Additional conformance scenarios that are deliberately stronger than both
  implementation test suites, especially adversarial peer and temporary network
  failure cases.

Baseline scenarios are not treated as "already satisfied". They are cataloged
so the black-box harness can implement and report equivalent behavior. Until a
baseline scenario has executable harness logic, the report must show it as
`skipped` or `unsupported`, never silently omit it.
