# AGENTS — maintaining the API2Convert Go SDK

This SDK is **hand-written** (not generated from OpenAPI) and kept in sync with the API by a human
**or an AI agent**. It is one of the official ports (PHP, Python, Java, Node.js, Go) that all
implement the same language-agnostic contract in [`docs/SDK_CONTRACT.md`](docs/SDK_CONTRACT.md).

## Why hand-written

The conversion flow is multi-step (create → upload → poll → download) and the **upload step is not in
the OpenAPI spec at all**, so a generator cannot produce a usable client. We optimise for a
junior-friendly surface — one-call `Convert()` — and use AI to keep it current.

## Repo layout

| Path | What it is |
| --- | --- |
| `client.go`, `options.go` | The client + `Convert` / `ConvertAsync` / `Download` façade and functional options. **Hand-authored.** |
| `result.go` | `ConversionResult` + `FileDownload` helpers. **Hand-authored.** |
| `uploader.go` | Streaming multipart upload to the per-job server. **Hand-authored** (not in the spec). |
| `webhook.go` | Webhook HMAC verification + parsing. **Hand-authored.** |
| `resources.go` | One type per API tag (Jobs, Conversions, Presets, Stats, Contracts). **Derived** from the spec. |
| `job.go`, `models.go`, `enums.go` | Typed structs (`*FromMap` factories) / enums. **Derived** from the spec. |
| `transport.go`, `httpsender.go`, `nethttpsender.go` | Transport: auth, retries/backoff, error mapping, redirect policy, the `HttpSender` seam. |
| `errors.go` | The typed error hierarchy. |
| `data.go` | Tolerant JSON hydration helpers (never panic on a surprising payload). |
| `openapi/api2convert.openapi.json` | **Committed spec snapshot** the SDK targets — the diff baseline (keep md5-identical to siblings). |
| `docs/SDK_CONTRACT.md` | The fixed, language-agnostic public surface + semantics (keep md5-identical to siblings). |
| `*_test.go` | Offline unit tests (fake `HttpSender`). **The guardrail.** |
| `security/` | The independent security suite (real loopback servers). **The redirect/leak guardrail.** |
| `live/` | Live conformance (build tag `live`; auto-skips without `API2CONVERT_API_KEY`). |

## How to update the SDK to a new API version

1. **Refresh the snapshot.** Overwrite `openapi/api2convert.openapi.json` from
   `https://api.api2convert.com/v2/openapi.json` (or `/v2/schema`) and `git diff` it. Keep it
   md5-identical to the sibling SDKs.
2. **Diff it** — new/removed/renamed operations, new fields, new enum values.
3. **Update the DERIVED layer to match the diff, and nothing else:**
   - New/changed fields → update the relevant struct in `models.go` / `job.go` + its `*FromMap`.
   - New operation → add a method on the matching resource in `resources.go` (mirror the style).
   - New input/output target types → extend `enums.go`.
4. **Do NOT change the hand-authored public API** (`Convert`, `ConvertAsync`, `Download`, upload,
   polling, webhook verification, error types) unless `docs/SDK_CONTRACT.md` changes first. If a real
   product change requires it, update the contract in the same change and bump the **major** version.
5. **Format, vet and test (the guardrail):**
   ```sh
   make check     # gofmt + go vet + unit tests + security suite — all must pass
   ```
   Add or update a test for any new behavior. Keep the live conformance test runnable.
6. **Record + version.** Add a `docs/CHANGELOG.md` entry and bump `Version` in `version.go` per SemVer
   (additive spec change → minor; breaking public-surface change → major). Tag `vX.Y.Z`.

## Module path & versioning (Go semantic import versioning)

- The version tracks the shared contract version (lockstep with the PHP/Python/Java/Node SDKs), so the
  Go SDK is at **major 10**. Go's semantic import versioning therefore requires the module path to
  carry a **`/v10` suffix**: the module path is `github.com/QaamGo/api2convert-go/v10` and consumers
  import `…/api2convert-go/v10`. Without the suffix, `go get …@v10.x` fails at go.mod parse and the
  proxy never serves the tag. `release.yml` guards this (the "module path suffix matches major
  version" step); keep it.
- A breaking public-surface change that bumps the **major** to 11 must bump the module path to `/v11`
  and update every import — such changes break already-published `v10.x` consumers, so batch them.

## Guarantees to uphold (don't break these)

- **Never commit a real API key, token or secret** — not in source, tests, fixtures, examples, CI
  files or commit messages, and never publish one anywhere. Keys come only from environment variables
  (`API2CONVERT_API_KEY`) or masked/protected CI variables; tests use obvious fakes (`test-key`,
  `whsec_test`, …). The SDK must never log or expose a key/token in errors. Secret-scan before any
  release.
- **The contract is law.** Public method names, signatures and semantics match `docs/SDK_CONTRACT.md`
  across every SDK language, adapted only to Go idiom (see divergences below).
- **Upload uses the per-job `X-Oc-Token`, never the account key.** There is a test for this.
- **Secret-bearing requests never follow redirects.** The key/token/download-password ride in custom
  `X-Oc-*` headers that Go's default client would forward across hosts. Only the no-secret download
  path follows redirects (a second `http.Client`). `security/` proves the guarantee with real servers.
- **`Convert()` stays one call** for the common case (path/URL/reader → `to` → `Save()`).
- **Transient failures retry; failures surface as typed errors.** Never leak a raw transport error
  (wrap it in `*NetworkError`). A non-idempotent `POST` is never blindly retried.
- **Go 1.22+, zero runtime dependencies (stdlib only).** Don't add runtime deps; `go.mod` must have no
  `require` block. Dev tools (staticcheck) run via `go run …@pinned`, never as a module dependency.

## Go-idiom divergences from the contract

The contract fixes names and semantics; these are the only places Go deviates, all for idiom:

- **Every I/O method takes a `context.Context` first argument** (Go's cancellation idiom), e.g.
  `Convert(ctx, input, to, opts...)`. `Download()` and option construction do no I/O (no ctx) until
  `Save`/`Contents`.
- **The "extra" `Convert` controls are functional options** (`WithConversionOptions`, `WithCategory`,
  `WithDownloadPassword`, `WithOutputIndex`, `WithFilename`, `WithCallback`, `WithConvertTimeout`),
  kept separate from the open-ended conversion-options map exactly as the contract requires. The
  client's timeout knob is `WithConvertTimeout` (the client-construction one is `WithTimeout`).
- **`New` returns `(*Client, error)`** — the Go-honest form of the siblings' throwing constructor.
- **Errors are typed structs** matching by `errors.As`; the base is the `Api2ConvertError` interface,
  any HTTP error also satisfies `HTTPError`. `ConversionTimeoutError` (not `TimeoutError`).
- **Job status predicates are methods** (`IsCompleted()`, `IsFailed()`, `IsCanceled()`,
  `IsTerminal()`), not precomputed fields; models keep `Raw`. Nullable numbers use `*int64`; nullable
  strings use the empty string (consult `Raw` for the exact JSON value).
- **The security suite is a black-box package** (`security/`, `package security_test`) runnable in
  isolation via `go test ./security/...` — the Go analog of the siblings' isolated security suites.

### Additive Go-only hardening (beyond the contract)

- **`WithMaxDownloadBytes(n)`** caps the bytes read from a download (unlimited by default), turning an
  oversized response into a `*NetworkError`. This is additive hardening, not part of the shared
  contract — see `SECURITY.md`. It is intentional extra public surface, not drift.

## Conventions

- Models parse defensively via `data.go` (tolerate missing/extra fields; never panic during
  hydration). `Job.Raw` keeps the full response.
- Resource methods are thin: build the request, call the transport, hydrate a model.
- Keep the README quickstart copy-pasteable; if you change the happy path, update the README example.
