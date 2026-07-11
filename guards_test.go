package api2convert_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// TestRedirectOnJSONPathIsError proves a 3xx on the authenticated (no-redirect)
// JSON path is a typed error, not a zero-value model with a nil error (M2).
func TestRedirectOnJSONPathIsError(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddJSON(302, map[string]any{}) // empty-body redirect on the no-redirect client

	_, err := tc.Client.Jobs().Get(context.Background(), "j1")
	if err == nil {
		t.Fatal("a 3xx on the JSON path must be an error, not a zero-value job")
	}
	var ne *api2convert.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T (%v), want *NetworkError", err, err)
	}
}

// TestZeroValueDownloadReturnsErrorNotPanic proves user-constructed result/download
// struct literals (nil transport) return a typed error rather than nil-panicking
// (L-nil-transport-panic).
func TestZeroValueDownloadReturnsErrorNotPanic(t *testing.T) {
	var d api2convert.FileDownload
	if _, err := d.Save(context.Background(), "out.bin"); err == nil {
		t.Fatal("Save on a zero-value FileDownload should return an error")
	}
	if _, err := d.Contents(context.Background()); err == nil {
		t.Fatal("Contents on a zero-value FileDownload should return an error")
	}
	var r api2convert.ConversionResult
	if _, err := r.Download(); err == nil {
		t.Fatal("Download on a zero-value ConversionResult should return an error")
	}
}

// TestOutputIndexOutOfRangeMessageWhenOutputsExist proves the error message no
// longer claims "no output files" when the job did produce some (L-output-index-msg).
func TestOutputIndexOutOfRangeMessageWhenOutputsExist(t *testing.T) {
	res, _ := completedURLConvert(t, api2convert.WithOutputIndex(9)) // job has 2 outputs
	_, err := res.Output()
	if err == nil {
		t.Fatal("an out-of-range output index should error")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("message should say the index is out of range, got: %v", err)
	}
}

// TestAcceptHeaderPerPath proves a binary download advertises Accept: */* while a
// JSON control-plane request advertises application/json (info-batch).
func TestAcceptHeaderPerPath(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("bin")).AddJSON(200, map[string]any{"id": "x"})

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	if _, err := tc.Client.Download(out).Contents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := tc.HTTP.At(0).H("Accept"); got != "*/*" {
		t.Fatalf("download Accept = %q, want */*", got)
	}
	if _, err := tc.Client.Jobs().Get(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if got := tc.HTTP.At(1).H("Accept"); got != "application/json" {
		t.Fatalf("JSON Accept = %q, want application/json", got)
	}
}

// TestDownloadNilBodyIsErrorNotPanic proves a custom sender returning a nil Body
// yields a typed error rather than a nil-panic on the deferred Close (info-batch).
func TestDownloadNilBodyIsErrorNotPanic(t *testing.T) {
	c, err := api2convert.New("k", api2convert.WithHTTPSender(&bodySender{body: nil}), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	if _, err := c.Download(out).Contents(context.Background()); err == nil {
		t.Fatal("a nil response body should be an error, not a panic")
	}
}
