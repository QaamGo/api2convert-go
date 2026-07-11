# Security Policy

## Reporting a vulnerability

Please **do not** open a public GitHub issue for a security problem in this SDK.

Report it privately through GitHub's **"Report a vulnerability"** button under the repository's
_Security_ tab (private vulnerability reporting). If that is unavailable, use the support channels at
<https://www.api2convert.com>. Please avoid disclosing details publicly until a fix has been released.

## Secrets this SDK handles

The library handles three secrets on the caller's behalf — keep all of them out of source control and
configure them via environment variables or a secret manager:

- the **account API key** (`X-Api2convert-Api-Key`) — read from configuration or the `API2CONVERT_API_KEY`
  environment variable and sent only to the API host, never in a URL query string;
- the **per-job upload token** (`X-Api2convert-Token`) — used to authenticate uploads to the per-job upload
  server; the account key is **never** sent there;
- the **webhook signing secret** — used locally to verify callback signatures (HMAC-SHA256 over the
  raw request body, constant-time comparison via `crypto/hmac`'s `hmac.Equal`). The signature is
  delivered in the `X-Oc-Signature` header.

## Guarantees

- The SDK never logs a key/token and never places one in an error message.
- **A request that carries any secret in a custom header never follows HTTP redirects.** The account
  key (`X-Api2convert-Api-Key`), per-job upload token (`X-Api2convert-Token`) and download password
  (`X-Api2convert-Download-Password`) all ride in custom `X-Api2convert-*` headers. Go's `http.Client` follows redirects
  by default and its redirect handler forwards custom headers across a cross-host redirect (since Go
  1.8 it strips only `Authorization`/`Www-Authenticate`/`Cookie`/`Cookie2` on a domain change), so a
  redirect could otherwise forward the secret to another host. The SDK therefore routes every
  secret-bearing request through a no-redirect client (`CheckRedirect` returns
  `http.ErrUseLastResponse`). Only a plain, passwordless download (`GET output.uri`, which carries no
  secret) follows redirects, so storage/CDN URLs still resolve. A cross-host redirect test suite
  (`security/`) proves this against real loopback servers.
- A directory download uses a sanitized basename derived from the API-supplied filename
  (`path.Base` after stripping NUL and normalizing separators), so a malicious name (e.g.
  `../../evil`) cannot escape the target directory.
- Untrusted JSON is hydrated into structs/maps and never panics on a surprising payload; unknown
  fields are tolerated and preserved in `Raw`. (Go's `encoding/json` has no prototype-pollution
  analog.)
- The input-type URL matcher is anchored (`^https?://`) and, because Go's `regexp` is RE2, runs in
  linear time — ReDoS is impossible by construction.
- Transient failures are retried with capped, jittered backoff; a non-idempotent `POST` (no
  `Idempotency-Key`) is never blindly retried, so a transient error cannot create a duplicate job. A
  one-shot `io.Reader` upload body is non-replayable and is therefore sent at most once.
- A malformed API-supplied URL surfaces as the SDK's `*NetworkError`, not a raw parse error.

### Go-specific additive hardening

- **`WithMaxDownloadBytes(n)`** caps the number of bytes read from a download; a larger response
  yields a `*NetworkError` instead of an unbounded read. Disabled (unlimited) by default.

### Documented future hardening

- Rejecting an `http://` API base when TLS is enforced is not yet implemented (a placeholder test
  documents the gap). Configure the client with the default `https://` base URL.

## Never commit a real key

Not in source, tests, fixtures, examples, CI files or commit messages. Keys come only from
environment variables (`API2CONVERT_API_KEY`) or masked/protected CI variables; tests use obvious
fakes (`test-key`, `secret-key`, `whsec_test`, …). If a key is ever exposed, revoke and rotate it in
the API2Convert dashboard immediately.
