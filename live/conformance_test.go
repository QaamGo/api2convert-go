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
package live_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteJPG = "https://example-files.online-convert.com/raster%20image/jpg/example_small.jpg"

func liveClient(t *testing.T) *api2convert.Client {
	t.Helper()
	key := os.Getenv("API2CONVERT_API_KEY")
	if key == "" {
		t.Skip("live tests require API2CONVERT_API_KEY (export the behat default key to run)")
	}
	var opts []api2convert.Option
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		opts = append(opts, api2convert.WithBaseURL(base))
	}
	c, err := api2convert.New(key, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestConvertsRemoteImageToPNG(t *testing.T) {
	c := liveClient(t)
	res, err := c.Convert(context.Background(), remoteJPG, "png")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !res.Job.IsCompleted() {
		t.Fatalf("job not completed: %+v", res.Job.Status)
	}
	dst := filepath.Join(t.TempDir(), "out.png")
	if _, err := res.Save(context.Background(), dst); err != nil {
		t.Fatalf("save: %v", err)
	}
	fi, err := os.Stat(dst)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("expected a non-empty output file (size=%d err=%v)", fi.Size(), err)
	}
}

func TestInvalidTargetRaisesValidationError(t *testing.T) {
	c := liveClient(t)
	_, err := c.Convert(context.Background(), remoteJPG, "this-is-not-a-real-target")
	var ve *api2convert.ValidationError
	var cf *api2convert.ConversionFailedError
	// The API rejects an unknown target either at create time (validation) or as a
	// failed job; accept both as a correct, typed failure.
	if !errors.As(err, &ve) && !errors.As(err, &cf) {
		t.Fatalf("err = %T (%v), want *ValidationError or *ConversionFailedError", err, err)
	}
}

func TestListConversionsCatalog(t *testing.T) {
	c := liveClient(t)
	opts, err := c.Options(context.Background(), "png", "image")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if len(opts) == 0 {
		t.Skip("no options returned for png/image (catalog may vary); conversion path already covered")
	}
}
