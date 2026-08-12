# Changelog

All notable changes to the API2Convert Go SDK are documented here. The version is kept in lockstep
with the PHP/Python/Java/Node.js SDKs.

## [10.4.0] - 2026-08-12

### Changed

- **`JobMessage.Code` is now `*int64` (was `*int`).** The wire decoder (`nullableInt64`) already
  reads values past 2³¹, so the old narrowing conversion truncated on 32-bit builds. Code that
  formats the field (`%d`) or stores it in an `any` is unaffected; code that assigns it to an
  `*int` variable needs the type updated.

## [10.3.1] - 2026-07-12

- Ships the cloud-storage examples added to the README (READMEs are included in the module zip).
  No functional or API change from 10.3.0.

## [10.3.0] - 2026-07-12

### Added

- Cloud-storage connectors: typed `CloudInput` + `OutputTarget` (SDK contract D-5).
- On-brand `x-api2convert-*` request headers.

### Fixed

Fixes from an in-depth code review (behavior/robustness; no public API breaks):

- Large files no longer fail at the default timeout: the per-request timeout now bounds only the
  pre-body phase (connect / TLS / response headers) on streamed download & upload paths; the body
  transfer is governed by the caller's context.
- `Save` streams to a temp file and renames over the target only after a clean copy+close — it never
  truncates the target up front, never leaves a partial file, and never destroys a pre-existing
  complete file on failure; a mid-download read failure is now a typed `*NetworkError`, and a
  write-back error surfaced at `Close` is no longer dropped.
- Stats docs corrected (`filter` is `single`/`all`, never an API key); the account key is redacted
  from any wrapped transport error message.
- A 3xx on the authenticated JSON path is a typed error (was a zero-value model + nil error).
- Fixed an upload file-descriptor leak on early-failing uploads; bounded control-plane JSON reads;
  out-of-range float hydration yields nil instead of implementation-defined garbage.
- Small robustness fixes: nil-transport guards, `Save("/")` root handling, clearer out-of-range
  output message, `Accept: */*` on downloads, nil response-body guard, concurrency-safety godoc.
- Test-harness fidelity: the fake fails fast on a missing fixture, honors context cancellation,
  records the request timeout, and propagates body-construction errors.
- CI/tooling: `workflow_dispatch` enabled, non-EOL Go matrix, API key scoped to the live step,
  live build tag vetted, zero-dependency guarantee enforced; pinned staticcheck; release workflow
  guards the module-path/major-version match.

## [10.2.1] - 2026-07-08

- Lock-step version bump to keep all API2Convert SDKs on 10.2.1. No importable library changes since
  10.2.0 (the redirect / download-password hardening already shipped); added a runnable example per
  documented guide and expanded the live-conformance suite to seven canonical scenarios.

## [10.2.0] - 2026-07-06

- Initial public release of the API2Convert Go SDK. Module path
  `github.com/QaamGo/api2convert-go/v10` (Go semantic import versioning for v10).
