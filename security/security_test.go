// Package security_test is the api2convert SDK's INDEPENDENT security suite.
//
// Run it in isolation with:
//
//	go test ./security/...      (or: make test-security)
//
// It is a black-box package (it imports the SDK by its module path), so every
// guarantee is proven through the public API only. The redirect guarantees use
// REAL loopback HTTP servers (net/http/httptest): only a genuine cross-host 302
// can demonstrate that the transport does not forward an X-Oc-* secret header to
// the redirect target. Go's http.Client default redirect handler forwards custom
// headers across a cross-host redirect (since Go 1.8 it strips only
// Authorization/Www-Authenticate/Cookie/Cookie2 on a domain change), so the SDK
// must route secret-bearing requests through a no-redirect client — which this
// suite proves. Header/JSON/ReDoS checks use the injected fake sender, where a
// real round-trip adds nothing.
package security_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go"
	"github.com/QaamGo/api2convert-go/internal/testutil"
)

func ctx() context.Context { return context.Background() }

// realClient builds a client backed by the real net/http sender (so the two
// redirect clients are exercised), with retries off and an instant sleeper.
func realClient(t *testing.T, baseURL, apiKey string) *api2convert.Client {
	t.Helper()
	opts := []api2convert.Option{
		api2convert.WithMaxRetries(0),
		api2convert.WithSleeper(func(context.Context, time.Duration) error { return nil }),
	}
	if baseURL != "" {
		opts = append(opts, api2convert.WithBaseURL(baseURL))
	}
	if apiKey == "" {
		apiKey = "secret-key"
	}
	c, err := api2convert.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// -------------------- secret hygiene (fake sender) --------------------

func TestSecretNeverAppearsInErrorMessage(t *testing.T) {
	const secret = "sk_live_super_secret_value_123"
	fake := &testutil.FakeSender{}
	c, err := api2convert.New(secret, api2convert.WithHTTPSender(fake), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddJSON(401, map[string]any{"message": "Invalid API key."})

	_, gotErr := c.Jobs().Get(ctx(), "job-x")
	if gotErr == nil {
		t.Fatal("expected an authentication error")
	}
	if strings.Contains(gotErr.Error(), secret) {
		t.Fatal("the API key must never leak into an error message")
	}
	if strings.Contains(fmt.Sprintf("%+v", gotErr), secret) {
		t.Fatal("the API key must never leak into a verbose error rendering")
	}
	// ...but it WAS sent as the auth header (the request was genuinely authenticated).
	if got := fake.At(0).H("X-Oc-Api-Key"); got != secret {
		t.Fatalf("auth header = %q, want the secret", got)
	}
}

func TestAPIKeyNeverAppearsInURLOrQueryString(t *testing.T) {
	const key = "sk_live_in_url_check"
	fake := &testutil.FakeSender{}
	c, err := api2convert.New(key, api2convert.WithHTTPSender(fake))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddJSON(200, []any{}).AddJSON(200, []any{})

	if _, err := c.Options(ctx(), "jpg", "image"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Jobs().List(ctx(), "completed", 2); err != nil {
		t.Fatal(err)
	}

	keyInQuery := regexp.MustCompile(`(?i)[?&](api[-_]?key|apikey|key)=`)
	for _, req := range fake.Requests() {
		if strings.Contains(req.URL, key) {
			t.Fatalf("API key leaked into URL: %q", req.URL)
		}
		if keyInQuery.MatchString(req.URL) {
			t.Fatalf("URL carries a key-like query param: %q", req.URL)
		}
	}
}

// -------------------- redirect policy (real loopback servers) --------------------

func TestAccountKeyNotForwardedAcrossCrossHostRedirect(t *testing.T) {
	evil := testutil.StartServer(testutil.Respond(200, "grabbed"))
	t.Cleanup(evil.Close)
	api := testutil.StartServer(testutil.RedirectTo(evil.URL + "/steal"))
	t.Cleanup(api.Close)

	c := realClient(t, api.URL+"/v2", "")
	_, _ = c.Jobs().Get(ctx(), "j") // the un-followed 302 yields no usable body

	if evil.Hits() != 0 {
		t.Fatal("the account key must never be forwarded to a redirect target")
	}
	if api.Hits() != 1 {
		t.Fatalf("api server hits = %d, want 1", api.Hits())
	}
}

func TestUploadUsesJobTokenNotAccountKeyAndDoesNotRedirect(t *testing.T) {
	evil := testutil.StartServer(testutil.Respond(200, "grabbed"))
	t.Cleanup(evil.Close)
	uploadSrv := testutil.StartServer(testutil.RedirectTo(evil.URL + "/steal"))
	t.Cleanup(uploadSrv.Close)

	c := realClient(t, "", "")
	job := api2convert.JobFromMap(map[string]any{
		"id": "job-9", "token": "tok-abc", "server": uploadSrv.URL,
		"status": map[string]any{"code": "incomplete"},
	})
	_, _ = c.Jobs().Upload(ctx(), job, []byte("hello"))

	seen := uploadSrv.Headers()
	if len(seen) == 0 {
		t.Fatal("upload server received no request")
	}
	if seen[0].Get("X-Oc-Token") != "tok-abc" {
		t.Fatalf("upload token header = %q", seen[0].Get("X-Oc-Token"))
	}
	if seen[0].Get("X-Oc-Api-Key") != "" {
		t.Fatal("the account key must never reach the upload server")
	}
	if evil.Hits() != 0 {
		t.Fatal("an authenticated upload must not follow a redirect")
	}
}

func TestDownloadPasswordNotForwardedAcrossCrossHostRedirect(t *testing.T) {
	evil := testutil.StartServer(testutil.Respond(200, "grabbed"))
	t.Cleanup(evil.Close)
	storage := testutil.StartServer(testutil.RedirectTo(evil.URL + "/steal"))
	t.Cleanup(storage.Close)

	c := realClient(t, "", "")
	out := api2convert.OutputFileOf("o", storage.URL+"/f.pdf", "")
	_, _ = c.Download(out, "s3cret").Contents(ctx())

	if evil.Hits() != 0 {
		t.Fatal("the download password must never be forwarded to a redirect target")
	}
	// The password WAS sent to the intended storage host (the request was real).
	if seen := storage.Headers(); len(seen) == 0 || seen[0].Get("X-Oc-Download-Password") != "s3cret" {
		t.Fatal("the download password should reach the intended storage host")
	}
}

func TestPasswordlessDownloadFollowsStorageRedirect(t *testing.T) {
	storage := testutil.StartServer(testutil.Respond(200, "REDIRECTED-BYTES"))
	t.Cleanup(storage.Close)
	dl := testutil.StartServer(testutil.RedirectTo(storage.URL + "/file"))
	t.Cleanup(dl.Close)

	c := realClient(t, "", "")
	out := api2convert.OutputFileOf("o", dl.URL+"/result.bin", "")
	data, err := c.Download(out).Contents(ctx())
	if err != nil {
		t.Fatalf("passwordless download failed: %v", err)
	}
	if string(data) != "REDIRECTED-BYTES" {
		t.Fatalf("contents = %q", data)
	}
	if storage.Hits() != 1 {
		t.Fatalf("storage hits = %d, want 1", storage.Hits())
	}
}

func TestPasswordProtectedDownloadDoesNotFollowButPasswordlessDoes(t *testing.T) {
	plainTarget := testutil.StartServer(testutil.Respond(200, "REACHED"))
	t.Cleanup(plainTarget.Close)
	plainHop := testutil.StartServer(testutil.RedirectTo(plainTarget.URL + "/x"))
	t.Cleanup(plainHop.Close)
	pwTarget := testutil.StartServer(testutil.Respond(200, "REACHED"))
	t.Cleanup(pwTarget.Close)
	pwHop := testutil.StartServer(testutil.RedirectTo(pwTarget.URL + "/x"))
	t.Cleanup(pwHop.Close)

	c := realClient(t, "", "")

	data, err := c.Download(api2convert.OutputFileOf("o", plainHop.URL+"/f", "")).Contents(ctx())
	if err != nil || string(data) != "REACHED" {
		t.Fatalf("passwordless download: data=%q err=%v", data, err)
	}
	if plainTarget.Hits() != 1 {
		t.Fatalf("passwordless redirect target hits = %d, want 1", plainTarget.Hits())
	}

	_, _ = c.Download(api2convert.OutputFileOf("o", pwHop.URL+"/f", ""), "pw").Contents(ctx())
	if pwTarget.Hits() != 0 {
		t.Fatalf("password-protected download must not follow the redirect: target hits = %d", pwTarget.Hits())
	}
}

func TestMalformedDownloadURISurfacesAsNetworkError(t *testing.T) {
	c := realClient(t, "", "")
	out := api2convert.OutputFileOf("o", "https://exa mple.com/a b c", "f.pdf")
	_, err := c.Download(out).Contents(ctx())
	var ne *api2convert.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T (%v), want *NetworkError (no raw url parse error)", err, err)
	}
}

// -------------------- filesystem safety (fake sender) --------------------

func TestTraversalFilenameReducedToBasename(t *testing.T) {
	dir := t.TempDir()
	tc := testutil.NewTestClient()
	tc.HTTP.AddRaw(200, []byte("X"))

	out := api2convert.OutputFileOf("", "https://dl/x", "../../../etc/evil")
	path, err := tc.Client.Download(out).Save(ctx(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "evil"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "..", "..", "etc", "evil")); err == nil {
		t.Fatal("a traversal filename escaped the target directory")
	}
}

func TestMaxDownloadBytesRejectsOversizedBody(t *testing.T) {
	tc := testutil.NewTestClient(api2convert.WithMaxDownloadBytes(4))
	tc.HTTP.AddRaw(200, []byte("far more than four bytes"))

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	_, err := tc.Client.Download(out).Contents(ctx())
	var ne *api2convert.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T, want *NetworkError for an oversized download", err)
	}
}

func TestRejectsHTTPBaseWhenTLSEnforced(t *testing.T) {
	t.Skip("future hardening: reject an http:// API base when TLS is enforced — see SECURITY.md")
}

// -------------------- webhook signature verification --------------------

func TestWebhookSignatureVerification(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"job-1","status":{"code":"completed"}}`)
	sig := testutil.Sign(string(payload), secret)

	t.Run("accepts a valid signature", func(t *testing.T) {
		event, err := api2convert.Webhooks().ConstructEvent(payload, sig, secret)
		if err != nil || event.Job.ID != "job-1" {
			t.Fatalf("event=%+v err=%v", event, err)
		}
	})
	t.Run("rejects a tampered payload", func(t *testing.T) {
		_, err := api2convert.Webhooks().ConstructEvent(append(payload, ' '), sig, secret)
		if err == nil {
			t.Fatal("tampered payload must be rejected")
		}
	})
	t.Run("rejects an equal-length wrong signature (constant-time, no crash)", func(t *testing.T) {
		wrong := strings.Repeat("f", len(sig))
		_, err := api2convert.Webhooks().ConstructEvent(payload, wrong, secret)
		if err == nil {
			t.Fatal("equal-length wrong signature must be rejected")
		}
	})
	t.Run("empty secret is a deliberate verification bypass", func(t *testing.T) {
		event, err := api2convert.Webhooks().ConstructEvent(payload, "", "")
		if err != nil || event.Job.ID != "job-1" {
			t.Fatalf("event=%+v err=%v", event, err)
		}
	})
}

// -------------------- untrusted-JSON hardening --------------------

func TestHostileJSONHydratesWithoutPanicAndToleratesUnknownFields(t *testing.T) {
	// Go's encoding/json into structs/maps has no prototype-pollution analog;
	// a __proto__/constructor payload is just data. The guarantee here is that a
	// hostile/surprising payload hydrates without panicking and unknown fields are
	// tolerated (kept in Raw), never rejected.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("hydrating a hostile payload panicked: %v", r)
		}
	}()
	malicious := []byte(`{"__proto__":{"polluted":true},"constructor":{"prototype":{"x":1}},"id":"job-1","surprise_field":[1,2,3],"status":{"code":"completed"}}`)

	event, err := api2convert.Webhooks().Parse(malicious)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Job.ID != "job-1" || !event.Job.IsCompleted() {
		t.Fatalf("job = %+v", event.Job)
	}
	if event.Job.Raw["surprise_field"] == nil {
		t.Fatal("unknown fields must be tolerated and preserved in Raw")
	}
}

// -------------------- ReDoS / anchored classifier (RE2) --------------------

func TestInputClassifierIsAnchoredAndLinear(t *testing.T) {
	// Go's regexp is RE2 (linear time, no catastrophic backtracking), so ReDoS is
	// impossible by construction. We prove the SDK's URL classifier is anchored and
	// fast through the public API: a pathological "almost-URL" is treated as a
	// local path (an upload attempt), not a remote input, and classification is
	// effectively instant.
	pathological := "http" + strings.Repeat("p", 100_000) + "x" // not a URL

	tc := testutil.NewTestClient()
	// A staged create response, in case classification (wrongly) treated it as a path.
	tc.HTTP.AddJSON(201, map[string]any{
		"id": "job-1", "token": "tok", "server": "https://up/v2",
		"status": map[string]any{"code": "incomplete"},
	})

	start := time.Now()
	_, err := tc.Client.ConvertAsync(ctx(), pathological, "png")
	elapsed := time.Since(start)

	// Classified as a local path -> upload -> stat fails -> "Input file not found".
	if err == nil || !strings.Contains(err.Error(), "Input file not found") {
		t.Fatalf("pathological input should be treated as a local path, got err=%v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("classification took %v (expected linear/instant)", elapsed)
	}

	// A real URL is still classified as a remote input (started immediately).
	tc2 := testutil.NewTestClient()
	tc2.HTTP.AddJSON(201, map[string]any{"id": "job-2", "status": map[string]any{"code": "downloading"}})
	if _, err := tc2.Client.ConvertAsync(ctx(), "https://example.com/x", "png"); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = tc2.HTTP.At(0).JSON(&body)
	if body["process"] != true {
		t.Fatalf("a real URL must be classified as a remote input: %v", body)
	}
}
