# Changelog

All notable changes to the API2Convert Go SDK are documented here. The version is kept in lockstep
with the PHP/Python/Java/Node.js SDKs.

## 10.2.0

Initial release of the official Go SDK — a feature-equivalent port of the shared
[SDK contract](SDK_CONTRACT.md).

- One-call `Convert` / `ConvertAsync` over the full job lifecycle (create → upload → start → poll →
  download), plus `Download` and `Options`.
- Resources: `Jobs` (Create/Get/List/Update/Start/Cancel/AddInput/Upload/Outputs/Wait),
  `Conversions` (List/Options), `Presets` (List/Create/Get/Update/Delete), `Stats` (Day/Month/Year),
  `Contracts`.
- Webhook verification (`Webhooks().ConstructEvent` / `Parse`, HMAC-SHA256, constant-time).
- Typed error hierarchy (`Api2ConvertError` / `HTTPError` + concrete types), automatic retries with
  jittered backoff honoring `Retry-After`, and floored/capped job polling.
- `context.Context`-first API, functional options, zero third-party runtime dependencies.
- Secret-safe transport: no secret in URLs or errors; secret-bearing requests never follow redirects
  (proven by the independent `security/` suite against real loopback servers).
- Go-only additive hardening: `WithMaxDownloadBytes` caps download size.
