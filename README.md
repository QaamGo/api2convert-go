# API2Convert Go SDK

The official Go SDK for the [API2Convert](https://www.api2convert.com) file-conversion API —
convert, compress and transform files with one call. It is one of the official ports (PHP, Python,
Java, Node.js, Go) that all implement the same [SDK contract](docs/SDK_CONTRACT.md).

- Zero third-party runtime dependencies (standard library only).
- `context.Context`-first, functional options, typed errors (`errors.As`).
- Automatic retries with jittered backoff; job polling with a floored interval and capped timeout.
- Streaming multipart upload and streaming download.
- Secret-safe by design: the account key, per-job token and download password never follow a
  redirect and never appear in a URL or an error message (see [SECURITY.md](SECURITY.md)).

## Install

```sh
go get github.com/QaamGo/api2convert-go
```

Requires Go 1.22+.

## Quick start

```go
package main

import (
	"context"
	"log"

	api2convert "github.com/QaamGo/api2convert-go"
)

func main() {
	// The API key falls back to the API2CONVERT_API_KEY env var when "".
	client, err := api2convert.New("YOUR_API_KEY")
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.Convert(context.Background(), "photo.png", "jpg")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := res.Save(context.Background(), "photo.jpg"); err != nil {
		log.Fatal(err)
	}
}
```

`Convert` accepts a local path (`string`), a public URL (`^https?://`), in-memory bytes (`[]byte`),
or an `io.Reader`.

## More examples

Given a `client` and a `ctx context.Context`, each variation is a single call.
(Error handling is elided here — check it as in the quick start above.)

```go
// From a URL (fetched server-side).
res, err := client.Convert(ctx, "https://example.com/photo.png", "jpg")

// With conversion options (discover them via client.Options); saving into a
// directory keeps the API's filename.
res, err = client.Convert(ctx, "photo.png", "jpg",
	api2convert.WithConversionOptions(map[string]any{"quality": 85, "width": 1280, "height": 720}))
res.Save(ctx, "out/")

// Password-protected output — remembered and applied automatically on download.
res, err = client.Convert(ctx, "statement.docx", "pdf",
	api2convert.WithDownloadPassword("hunter2"))

// Async with a webhook callback (returns once the job is started).
job, err := client.ConvertAsync(ctx, "movie.mov", "mp4",
	api2convert.WithCallback("https://your-app.example.com/webhooks/api2convert"))
```

## Typed errors

Every failure the SDK returns satisfies `api2convert.Api2ConvertError`. Match specific failures with
`errors.As`:

```go
res, err := client.Convert(ctx, "in.psd", "png")
switch {
case err == nil:
	// ok
default:
	var rl *api2convert.RateLimitError
	var ve *api2convert.ValidationError
	var cf *api2convert.ConversionFailedError
	switch {
	case errors.As(err, &rl):
		log.Printf("rate limited; retry after %v", rl.RetryAfter)
	case errors.As(err, &ve):
		log.Printf("invalid request: %v", ve)
	case errors.As(err, &cf):
		log.Printf("job %s failed: %v", cf.Job.ID, cf.Errors())
	default:
		log.Printf("error: %v", err)
	}
}
```

Any HTTP error (status ≥ 400) also satisfies `api2convert.HTTPError` (`Status()`, `RequestID()`,
`Body()`).

## Webhooks

Verify a signed callback (HMAC-SHA256 over the raw body, delivered in the `X-Oc-Signature` header):

```go
event, err := api2convert.Webhooks().ConstructEvent(rawBody, signatureHeader, "YOUR_WEBHOOK_SECRET")
if err != nil {
	// invalid signature — reject
}
job := event.Job
```

`api2convert.Webhooks()` needs no configured client. Pass an empty secret to skip verification, or
use `Parse` for accounts without signed webhooks enabled.

## Full lifecycle control

`Convert` is built on the resources, which you can use directly for compound jobs, presets, custom
polling or job chaining:

```go
job, _ := client.Jobs().Create(ctx, map[string]any{
	"conversion": []any{map[string]any{"target": "pdf"}},
	"process":    false,
})
_, _ = client.Jobs().Upload(ctx, *job, "invoice.docx")
_, _ = client.Jobs().Start(ctx, job.ID)
done, _ := client.Jobs().Wait(ctx, job.ID, 0, true) // 0 = default poll timeout
outputs := done.Output
_ = outputs
```

Also available: `client.Conversions()`, `client.Presets()`, `client.Stats()`, `client.Contracts()`,
and `client.Options(ctx, target, category...)` to discover a target's options.

## Configuration

```go
client, _ := api2convert.New("KEY",
	api2convert.WithBaseURL("https://api.api2convert.com/v2"),
	api2convert.WithTimeout(30*time.Second),
	api2convert.WithMaxRetries(2),
	api2convert.WithPollInterval(1*time.Second),
	api2convert.WithPollMaxInterval(5*time.Second),
	api2convert.WithPollTimeout(300*time.Second),
	api2convert.WithMaxDownloadBytes(0), // 0 = unlimited (Go-only hardening)
)
```

## Testing

```sh
make test           # offline unit tests + the hermetic security suite (no key)
make test-security  # the independent security suite, run in isolation
make test-live      # live conformance — requires API2CONVERT_API_KEY
make check          # fmt + vet + test + test-security
```

### Running live tests

The live suite hits the real API and consumes quota. It is gated by the `live` build tag **and**
skips unless `API2CONVERT_API_KEY` is set. Never commit a key — supply it at run time:

```sh
API2CONVERT_API_KEY=<your key> make test-live
# optionally target another environment:
API2CONVERT_API_KEY=<key> API2CONVERT_BASE_URL=https://api.api2convert.com/v2 make test-live
```

## License

[MIT](LICENSE) © Qaamgo Media GmbH
