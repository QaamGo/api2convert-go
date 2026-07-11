package security_test

import (
	"fmt"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// assertNoSecret fails if secret appears in the error's message or its verbose
// (%+v) rendering.
func assertNoSecret(t *testing.T, err error, secret string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("secret leaked into error message: %q", err.Error())
	}
	if strings.Contains(fmt.Sprintf("%+v", err), secret) {
		t.Fatal("secret leaked into the verbose error rendering")
	}
}

// TestPathSegmentsArePercentEncoded proves a caller-supplied id (which may arrive
// from an untrusted webhook payload) can never inject extra path segments, a
// query, a fragment or a traversal into the request URL (M1). Every sibling SDK
// percent-encodes path segments; this pins the Go behavior.
func TestPathSegmentsArePercentEncoded(t *testing.T) {
	fake := &testutil.FakeSender{}
	c, err := api2convert.New("test-key", api2convert.WithHTTPSender(fake))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddJSON(200, map[string]any{})

	const hostile = "a/b?c#d/../../presets/p9"
	_, _ = c.Jobs().Get(ctx(), hostile)

	url := fake.Last().URL
	const marker = "/jobs/"
	i := strings.Index(url, marker)
	if i < 0 {
		t.Fatalf("unexpected request URL: %q", url)
	}
	// After the last structural "/jobs/", the id must be one opaque segment: no
	// raw separators survive to re-target the request.
	tail := url[i+len(marker):]
	for _, raw := range []string{"/", "?", "#"} {
		if strings.Contains(tail, raw) {
			t.Fatalf("id not encoded to a single segment; URL tail %q still contains %q", tail, raw)
		}
	}
	if !strings.Contains(url, "a%2Fb%3Fc%23d") {
		t.Fatalf("expected the id percent-encoded in the URL, got %q", url)
	}
}

// TestKeyRedactedFromTransportError proves the account key never survives in a
// wrapped transport (NetworkError) error, even when the underlying error text
// echoes a URL that contains it (mirroring net/http's *url.Error). The original
// secret-hygiene test only covered a 4xx API-error path; this covers the
// NetworkError path from a stats call (H4).
func TestKeyRedactedFromTransportError(t *testing.T) {
	const secret = "sk_live_leaky_transport_key"
	fake := &testutil.FakeSender{}
	c, err := api2convert.New(secret, api2convert.WithHTTPSender(fake), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddError(fmt.Errorf(`Get "https://api.api2convert.com/v2/stats/day/2026-07-01/%s": dial tcp: connection refused`, secret))

	_, gotErr := c.Stats().Day(ctx(), "2026-07-01", "all")
	assertNoSecret(t, gotErr, secret)
}

// TestKeyRedactedFromUploadTransportError covers the upload path's error wrapping,
// which the existing secret-hygiene tests never exercised (H4).
func TestKeyRedactedFromUploadTransportError(t *testing.T) {
	const secret = "sk_live_upload_path_key"
	fake := &testutil.FakeSender{}
	c, err := api2convert.New(secret, api2convert.WithHTTPSender(fake), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddError(fmt.Errorf(`Post "https://upload.example/upload-file/j1": %s`, secret))

	job := api2convert.Job{ID: "j1", Server: "https://upload.example", Token: "upload-token"}
	_, gotErr := c.Jobs().Upload(ctx(), job, []byte("hello"))
	assertNoSecret(t, gotErr, secret)
}
