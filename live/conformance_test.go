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
// Each test mirrors one documented example guide (the same catalog implemented by
// every api2convert SDK) and asserts the operation succeeds, so this file doubles
// as an executable tour of the SDK. The runnable single-purpose programs live in
// ../examples/. The final two tests are negative: an invalid target is a typed
// validation/conversion error, and a bad key is a typed auth error that never
// leaks the credential.
package live_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// Remote fixtures — small, stable public files served by online-convert.
const (
	remotePDF      = "https://example-files.online-convert.com/document/pdf/example.pdf"
	remotePNG      = "https://example-files.online-convert.com/raster%20image/png/example.png"
	remoteJPG      = "https://example-files.online-convert.com/raster%20image/jpg/example.jpg"
	remoteJPGSmall = "https://example-files.online-convert.com/raster%20image/jpg/example_small.jpg"
	remoteMP4      = "https://example-files.online-convert.com/video/mp4/example.mp4"
	remoteWAV      = "https://example-files.online-convert.com/audio/wav/example.wav"
	remoteDOCX     = "https://example-files.online-convert.com/document/docx/example.docx"
	remoteZIP      = "https://example-files.online-convert.com/archive/zip/example.zip"
)

// onePxPNG is a minimal valid 1x1 PNG, written to disk to exercise the real
// multipart upload handshake (remote-URL inputs skip upload entirely).
var onePxPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// liveClient builds a client from the environment, or skips (passes) when no key
// is set. New reads the key you pass (falling back to API2CONVERT_API_KEY); we
// also honor API2CONVERT_BASE_URL so the same suite can target prod or beta.
func liveClient(t *testing.T) *api2convert.Client {
	t.Helper()
	if os.Getenv("API2CONVERT_API_KEY") == "" {
		t.Skip("live tests require API2CONVERT_API_KEY (export the behat default key to run)")
	}
	c, err := api2convert.New("", baseURLOption()...)
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

// mustComplete fails the test unless the job completed.
func mustComplete(t *testing.T, job api2convert.Job) {
	t.Helper()
	if !job.IsCompleted() {
		t.Fatalf("job %s should complete, got status %q", job.ID, job.Status.Code)
	}
}

// mustSaveNonEmpty saves the result and fails unless the file is non-empty.
func mustSaveNonEmpty(t *testing.T, ctx context.Context, res *api2convert.ConversionResult, name string) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), name)
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

// waitEmbeddedInputs creates a job with embedded remote inputs, starts it, and
// waits for completion.
func waitEmbeddedInputs(t *testing.T, ctx context.Context, c *api2convert.Client, payload map[string]any) *api2convert.Job {
	t.Helper()
	job, err := c.Jobs().Create(ctx, payload)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	done, err := c.Jobs().Wait(ctx, job.ID, 0, true)
	if err != nil {
		t.Fatalf("wait for job: %v", err)
	}
	return done
}

// 1. quickstart — convert a remote JPG to PNG, re-fetch the job, download. -----
func TestQuickstart(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "png")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)

	job, err := c.Jobs().Get(ctx, res.Job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.ID != res.Job.ID {
		t.Fatalf("get returned job %q, want %q", job.ID, res.Job.ID)
	}
	mustSaveNonEmpty(t, ctx, res, "out.png")
}

// 2. convert-files — browse the catalog (all + filtered), then convert. --------
func TestConvertFiles(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	all, err := c.Conversions().List(ctx, "", "", 1)
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("the catalog should be non-empty")
	}
	toPNG, err := c.Conversions().List(ctx, "", "png", 1)
	if err != nil {
		t.Fatalf("list png conversions: %v", err)
	}
	if len(toPNG) == 0 {
		t.Fatal("the catalog should list at least one conversion to png")
	}

	res, err := c.Convert(ctx, remoteJPG, "png")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
}

// 3. uploading-files — one-call upload + convert of a local file. --------------
func TestUploadingFiles(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	src := filepath.Join(t.TempDir(), "pixel.png")
	if err := os.WriteFile(src, onePxPNG, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	res, err := c.Convert(ctx, src, "png")
	if err != nil {
		t.Fatalf("convert uploaded file: %v", err)
	}
	mustComplete(t, res.Job)

	data, err := res.Contents(ctx)
	if err != nil {
		t.Fatalf("download output: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("converted output should be non-empty")
	}
}

// 4. job-lifecycle — manual create -> add input -> start -> wait -> outputs. ---
func TestJobLifecycle(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	jobs := c.Jobs()

	job, err := jobs.Create(ctx, map[string]any{
		"process":    false,
		"conversion": []any{map[string]any{"category": "image", "target": "png"}},
	})
	if err != nil {
		t.Fatalf("create staged job: %v", err)
	}
	if job.ID == "" {
		t.Fatal("a created job should have an id")
	}

	if _, err := jobs.AddInput(ctx, job.ID, map[string]any{"type": "remote", "source": remoteJPG}); err != nil {
		t.Fatalf("attach remote input: %v", err)
	}
	if _, err := jobs.Start(ctx, job.ID); err != nil {
		t.Fatalf("start job: %v", err)
	}

	done, err := jobs.Wait(ctx, job.ID, 0, true)
	if err != nil {
		t.Fatalf("wait for job: %v", err)
	}
	mustComplete(t, *done)

	outputs, err := jobs.Outputs(ctx, job.ID)
	if err != nil {
		t.Fatalf("fetch outputs: %v", err)
	}
	if len(outputs) == 0 {
		t.Fatal("job should have at least one output")
	}
}

// 5. add-watermark — two remote inputs (document + stamp) -> stamped PDF. ------
func TestAddWatermark(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	done := waitEmbeddedInputs(t, ctx, c, map[string]any{
		"process": true,
		"input": []any{
			map[string]any{"type": "remote", "source": remotePDF},
			map[string]any{"type": "remote", "source": remotePNG},
		},
		"conversion": []any{map[string]any{
			"category": "document",
			"target":   "pdf",
			"options":  map[string]any{"stamp": true, "alignment": "center"},
		}},
	})
	mustComplete(t, *done)
	if len(done.Output) == 0 {
		t.Fatal("watermark job should produce an output")
	}
}

// 6. create-thumbnails — render a PDF page to a PNG thumbnail. -----------------
func TestCreateThumbnails(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remotePDF, "thumbnail",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{
			"thumbnail_target": "png",
			"width":            300,
			"pages":            "first",
			"dpi":              150,
		}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
	mustSaveNonEmpty(t, ctx, res, "thumbnail.png")
}

// 7. compress-files — shrink a JPG with the compress operation. ----------------
func TestCompressFiles(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "compress",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{"compression_level": "high"}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
	mustSaveNonEmpty(t, ctx, res, "compressed.jpg")
}

// 8. create-archives — bundle two remote files into a ZIP. ---------------------
func TestCreateArchives(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	done := waitEmbeddedInputs(t, ctx, c, map[string]any{
		"process": true,
		"input": []any{
			map[string]any{"type": "remote", "source": remotePDF},
			map[string]any{"type": "remote", "source": remotePNG},
		},
		"conversion": []any{map[string]any{"category": "archive", "target": "zip"}},
	})
	mustComplete(t, *done)
	if len(done.Output) == 0 {
		t.Fatal("archive job should produce an output")
	}
}

// 9. create-hashes — compute a SHA-256 digest of a file. ----------------------
func TestCreateHashes(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteZIP, "sha256", api2convert.WithCategory("hash"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)

	data, err := res.Contents(ctx)
	if err != nil {
		t.Fatalf("download hash: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("hash output should be non-empty")
	}
}

// 10. extract-assets — pull embedded assets out of a document. -----------------
func TestExtractAssets(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteDOCX, "extract-assets", api2convert.WithCategory("operation"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
	if len(res.Outputs()) == 0 {
		t.Fatal("extract-assets job should produce at least one output")
	}
}

// 11. file-analysis — extract file metadata as JSON. --------------------------
func TestFileAnalysis(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "json", api2convert.WithCategory("metadata"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)

	data, err := res.Contents(ctx)
	if err != nil {
		t.Fatalf("download metadata: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("metadata output should be non-empty")
	}
}

// 12. compare-files — diff two images with the compare-image operation. -------
func TestCompareFiles(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	done := waitEmbeddedInputs(t, ctx, c, map[string]any{
		"process": true,
		"input": []any{
			map[string]any{"type": "remote", "source": remoteJPGSmall},
			map[string]any{"type": "remote", "source": remoteJPG},
		},
		"conversion": []any{map[string]any{
			"category": "operation",
			"target":   "compare-image",
			"options":  map[string]any{"method": "ssim", "threshold": 5, "diff_color": "red"},
		}},
	})
	mustComplete(t, *done)
}

// 13. capture-website — screenshot a URL with the screenshot engine. ----------
func TestCaptureWebsite(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	done := waitEmbeddedInputs(t, ctx, c, map[string]any{
		"process": true,
		"input": []any{map[string]any{
			"type":   "remote",
			"source": "https://www.online-convert.com",
			"engine": "screenshot",
			"options": map[string]any{
				"screen_width":        1280,
				"screen_height":       1024,
				"device_scale_factor": 1,
			},
		}},
		"conversion": []any{map[string]any{"category": "image", "target": "png"}},
	})
	mustComplete(t, *done)
	if len(done.Output) == 0 {
		t.Fatal("screenshot job should produce an output")
	}
}

// 14. audio-operations — transcode WAV to AAC with explicit options. ----------
func TestAudioOperations(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteWAV, "aac",
		api2convert.WithCategory("audio"),
		api2convert.WithConversionOptions(map[string]any{
			"audio_codec":   "aac",
			"audio_bitrate": 192,
			"channels":      "stereo",
			"frequency":     44100,
		}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
	mustSaveNonEmpty(t, ctx, res, "audio.aac")
}

// 15. image-operations — resize a JPG with the resize-image operation. --------
func TestImageOperations(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	res, err := c.Convert(ctx, remoteJPG, "resize-image",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{
			"width":           800,
			"height":          600,
			"resize_by":       "px",
			"resize_handling": "keep_aspect_ratio_crop",
		}))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	mustComplete(t, res.Job)
	mustSaveNonEmpty(t, ctx, res, "resized.jpg")
}

// 16. webhooks — async convert with a callback returns a started job with id. -
//
// A webhook receipt is not testable in CI, so we assert only that the async job
// starts and carries an id; we do not wait for the callback.
func TestWebhooks(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	job, err := c.ConvertAsync(ctx, remoteDOCX, "pdf",
		api2convert.WithCategory("document"),
		api2convert.WithCallback("https://your-app.example.com/api2convert/webhook"))
	if err != nil {
		t.Fatalf("convert async: %v", err)
	}
	if job.ID == "" {
		t.Fatal("an async job should have an id")
	}
}

// 17. presets — list presets (may be empty; assert the call returns a list). ---
func TestPresets(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	presets, err := c.Presets().List(ctx, "video", "mp4", "")
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	// An empty list is valid; assert only the type is a slice (len is defined).
	_ = len(presets)
}

// 18. statistics — usage for a recent month returns without error. ------------
func TestStatistics(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	month := time.Now().UTC().Format("2006-01")
	if _, err := c.Stats().Month(ctx, month, "all"); err != nil {
		t.Fatalf("stats month: %v", err)
	}
}

// 19. rate-limits — the contracts call returns without error. -----------------
func TestRateLimits(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	if _, err := c.Contracts().Get(ctx); err != nil {
		t.Fatalf("contracts: %v", err)
	}
}

// 20. authentication — an authenticated jobs.list returns a list, no error. ---
func TestAuthentication(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	jobs, err := c.Jobs().List(ctx, "", 1)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	_ = len(jobs)
}

// Negative 1. Validation error on an unknown target. --------------------------
//
// The API rejects an unknown target — either synchronously at create time
// (validation) or as a failed job. Both are typed errors matchable with errors.As.
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

// Negative 2. Authentication error, with no secret leak. ----------------------
//
// A bad key produces a typed *AuthenticationError carrying the HTTP status.
// Crucially, the SDK never puts a credential into an error message.
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
