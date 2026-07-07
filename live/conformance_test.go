//go:build live

// Package live_test holds the live conformance suite: it hits the real
// api2convert API and consumes quota, so it is gated two ways.
//
//   - The build tag `live` keeps it out of the default `go test ./...` and out of
//     `go vet`'s default build. Run it with: go test -tags live ./live/...
//
//   - Even then, each test skips unless API2CONVERT_API_KEY is set. To run it,
//     export the key first (the behat default key when validating against the
//     shared test account):
//
//     API2CONVERT_API_KEY=<key> go test -tags live -timeout 300s ./live/...
//
// Never commit a real key. The key is read only from the environment.
//
// Every test is written to read like an idiomatic usage example, so this file
// doubles as an executable tour of the SDK. The seven scenarios mirror the shared
// spec implemented by every api2convert SDK (php, python, java, go, nodejs,
// dotnet, ruby, rust):
//
//  1. TestConvertRemoteURLToPNG            — one-call convert of a URL
//  2. TestUploadLocalFileAndConvert        — multipart upload of a local file
//  3. TestConvertWithOptions               — apply conversion options
//  4. TestDiscoverConversionCatalog        — options/catalog discovery
//  5. TestManualJobLifecycleAndInspection  — create → input → start → wait
//  6. TestInvalidTargetIsATypedError       — validation error handling
//  7. TestAuthenticationErrorLeaksNoSecret — auth error, no key leak
package live_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// remoteJPG is a small, stable public image used as a remote input everywhere.
const remoteJPG = "https://example-files.online-convert.com/raster%20image/jpg/example_small.jpg"

// onePxPNG is a minimal valid 1×1 PNG, written to disk to exercise the real
// multipart upload handshake (remote-URL inputs skip upload entirely).
var onePxPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// liveClient builds a client from the environment, or skips (passes) when no key
// is set. This is the idiomatic construction: New reads the key you pass (falling
// back to API2CONVERT_API_KEY); here we also honor API2CONVERT_BASE_URL so the
// same suite can target prod or a beta host.
func liveClient(t *testing.T) *api2convert.Client {
	t.Helper()
	key := os.Getenv("API2CONVERT_API_KEY")
	if key == "" {
		t.Skip("live tests require API2CONVERT_API_KEY (export the behat default key to run)")
	}
	c, err := api2convert.New(key, baseURLOption()...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// baseURLOption honors an optional API2CONVERT_BASE_URL override (e.g. a beta
// environment); empty means the default prod host.
func baseURLOption() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}

// 1. One-call convert of a remote URL ---------------------------------------
//
// The simplest usage: hand Convert a URL and a target format. The SDK creates a
// server-side-fetch job, polls it to completion, and hands back a result you can
// Save straight to disk.
func TestConvertRemoteURLToPNG(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "png")
	if err != nil {
		t.Fatalf("convert remote URL: %v", err)
	}
	if !res.Job.IsCompleted() {
		t.Fatalf("job should complete, got status %q", res.Job.Status.Code)
	}

	dst := filepath.Join(t.TempDir(), "out.png")
	if _, err := res.Save(ctx, dst); err != nil {
		t.Fatalf("save output: %v", err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("output should be non-empty")
	}
}

// 2. Upload and convert a local file ----------------------------------------
//
// For a local path (or []byte / io.Reader), the SDK stages the job, streams the
// file to the per-job upload server (authenticated with the job's token, never
// your account key), starts it, polls, and downloads.
func TestUploadLocalFileAndConvert(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	src := filepath.Join(t.TempDir(), "pixel.png")
	if err := os.WriteFile(src, onePxPNG, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	res, err := c.Convert(ctx, src, "jpg")
	if err != nil {
		t.Fatalf("convert uploaded file: %v", err)
	}
	if !res.Job.IsCompleted() {
		t.Fatalf("uploaded job should complete, got status %q", res.Job.Status.Code)
	}

	bytes, err := res.Contents(ctx)
	if err != nil {
		t.Fatalf("download output: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatal("converted output should be non-empty")
	}
	// A JPEG starts with the SOI marker 0xFF 0xD8.
	if bytes[0] != 0xFF || bytes[1] != 0xD8 {
		t.Fatalf("output should be a JPEG (magic 0xFF 0xD8), got 0x%02X 0x%02X", bytes[0], bytes[1])
	}
}

// 3. Apply conversion options -----------------------------------------------
//
// Pass target-specific options through WithConversionOptions. They are kept
// strictly separate from the SDK's own controls, so an option key can never
// collide with an SDK argument. Discover the valid keys for a target with
// client.Options (see the next scenario); here we re-encode at a lower JPEG
// quality.
func TestConvertWithOptions(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "jpg",
		// Add "width": 64, "height": 64 to the map to resize.
		api2convert.WithConversionOptions(map[string]any{"quality": 50}))
	if err != nil {
		t.Fatalf("convert with options: %v", err)
	}
	if !res.Job.IsCompleted() {
		t.Fatalf("job should complete, got status %q", res.Job.Status.Code)
	}

	bytes, err := res.Contents(ctx)
	if err != nil {
		t.Fatalf("download output: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatal("converted output should be non-empty")
	}
}

// 4. Discover the conversion catalog ----------------------------------------
//
// Conversions().List and Options describe what the API can do — which targets
// exist and which options each accepts. Neither consumes conversion quota, so
// they are cheap to call before building a request.
func TestDiscoverConversionCatalog(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	// Which conversions target "jpg"? (category "", target "jpg", first page)
	conversions, err := c.Conversions().List(ctx, "", "jpg", 1)
	if err != nil {
		t.Fatalf("list conversions: %v", err)
	}
	if len(conversions) == 0 {
		t.Fatal("the catalog should list at least one conversion to jpg")
	}

	// The option schema for a target (type / enum / default / range per option).
	if _, err := c.Options(ctx, "png", "image"); err != nil {
		t.Fatalf("fetch option schema: %v", err)
	}
}

// 5. Drive the full job lifecycle by hand -----------------------------------
//
// Convert is built from these primitives. Driving them yourself unlocks
// compound/merge jobs, custom inputs, and step-by-step inspection: create a
// staged job, attach an input, start it, wait for completion, then inspect the
// job's status and output metadata.
func TestManualJobLifecycleAndInspection(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	jobs := c.Jobs()

	// Stage a job (process: false) so we can attach inputs before starting.
	job, err := jobs.Create(ctx, map[string]any{
		"process":    false,
		"conversion": []any{map[string]any{"target": "png"}},
	})
	if err != nil {
		t.Fatalf("create staged job: %v", err)
	}
	if job.ID == "" {
		t.Fatal("a created job should have an id")
	}

	// Attach a remote input, then start processing.
	if _, err := jobs.AddInput(ctx, job.ID, map[string]any{"type": "remote", "source": remoteJPG}); err != nil {
		t.Fatalf("attach remote input: %v", err)
	}
	if _, err := jobs.Start(ctx, job.ID); err != nil {
		t.Fatalf("start job: %v", err)
	}

	// Poll to a terminal status (0 = default poll timeout, throwOnFailure = true).
	finished, err := jobs.Wait(ctx, job.ID, 0, true)
	if err != nil {
		t.Fatalf("wait for job: %v", err)
	}
	if !finished.IsCompleted() {
		t.Fatalf("job should complete, got status %q", finished.Status.Code)
	}

	// Inspect the outputs — both from the finished job and via the outputs API.
	if len(finished.Output) == 0 {
		t.Fatal("job should have an output")
	}
	outputs, err := jobs.Outputs(ctx, job.ID)
	if err != nil {
		t.Fatalf("fetch outputs: %v", err)
	}
	if len(outputs) != len(finished.Output) {
		t.Fatalf("Outputs() should match the job's output list: got %d, want %d", len(outputs), len(finished.Output))
	}
	if finished.Output[0].URI == "" {
		t.Fatal("first output should have a non-empty download URI")
	}
}

// 6. Validation error on an unknown target ----------------------------------
//
// The API rejects an unknown target — either synchronously at create time
// (validation) or as a failed job. Both are typed errors you can match on with
// errors.As.
func TestInvalidTargetIsATypedError(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	_, err := c.Convert(ctx, remoteJPG, "this-is-not-a-real-target")
	var ve *api2convert.ValidationError
	var cf *api2convert.ConversionFailedError
	if !errors.As(err, &ve) && !errors.As(err, &cf) {
		t.Fatalf("err = %T (%v), want *ValidationError or *ConversionFailedError", err, err)
	}
}

// 7. Authentication error, with no secret leak ------------------------------
//
// A bad key produces a typed *AuthenticationError carrying the HTTP status.
// Crucially, the SDK never puts a credential into an error message — we assert
// the bogus key does not appear in the rendered error.
func TestAuthenticationErrorLeaksNoSecret(t *testing.T) {
	// Gate on the real key like the rest of the suite so this only runs when the
	// API is meant to be reachable — but authenticate with a bogus key below.
	_ = liveClient(t)
	ctx := context.Background()

	const bogusKey = "a2c-invalid-key-for-testing"
	c, err := api2convert.New(bogusKey, baseURLOption()...)
	if err != nil {
		t.Fatalf("build client with bogus key: %v", err)
	}

	_, err = c.Jobs().List(ctx, "", 1)
	var ae *api2convert.AuthenticationError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T (%v), want *AuthenticationError", err, err)
	}
	if s := ae.Status(); s != 401 && s != 403 {
		t.Fatalf("expected HTTP 401/403, got %d", s)
	}
	if strings.Contains(err.Error(), bogusKey) {
		t.Fatal("the error message must not leak the API key")
	}
}
