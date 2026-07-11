package api2convert_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func hdr(k, v string) http.Header { return http.Header{k: []string{v}} }

func TestRetries503ThenSucceedsOnIdempotentGet(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddJSON(503, map[string]any{"message": "busy"}).
		AddJSON(200, map[string]any{"id": "j"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if tc.HTTP.Count() != 2 {
		t.Fatalf("attempts = %d, want 2", tc.HTTP.Count())
	}
	// attempt 0 backoff: 0.5 * 2^0 = 0.5s, jitter disabled.
	if d := tc.Sleeper.Durations(); len(d) != 1 || d[0] != 500*time.Millisecond {
		t.Fatalf("backoff = %v, want [500ms]", d)
	}
}

func TestRetriesNetworkErrorThenSucceeds(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddError(errors.New("connection reset")).
		AddJSON(200, map[string]any{"id": "j"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if tc.HTTP.Count() != 2 {
		t.Fatalf("attempts = %d, want 2", tc.HTTP.Count())
	}
}

func TestWrapsExhaustedRetriesInNetworkError(t *testing.T) {
	sentinel := errors.New("dns failure")
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddError(sentinel).AddError(sentinel).AddError(sentinel)

	_, err := tc.Client.Jobs().Get(context.Background(), "j")
	var ne *api2convert.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T, want *NetworkError", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatal("wrapped NetworkError must expose the underlying cause via errors.Is")
	}
	if tc.HTTP.Count() != 3 {
		t.Fatalf("attempts = %d, want 3 (1 + 2 retries)", tc.HTTP.Count())
	}
}

func TestNeverRetriesBarePOSTOn5xx(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddJSON(503, map[string]any{"message": "down"})

	_, err := tc.Client.Jobs().Create(context.Background(), map[string]any{"conversion": []any{}})
	var se *api2convert.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("err = %T, want *ServerError", err)
	}
	if tc.HTTP.Count() != 1 {
		t.Fatalf("a bare POST must not be retried on 5xx: %d attempts", tc.HTTP.Count())
	}
}

func TestRetriesPOSTWithIdempotencyKeyOn5xx(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddJSON(503, map[string]any{"message": "down"}).
		AddJSON(201, map[string]any{"id": "job-1"})

	if _, err := tc.Client.Jobs().Create(context.Background(), map[string]any{"conversion": []any{}}, "idem-1"); err != nil {
		t.Fatal(err)
	}
	if tc.HTTP.Count() != 2 {
		t.Fatalf("a POST with Idempotency-Key should retry on 5xx: %d attempts", tc.HTTP.Count())
	}
}

func TestRetriesPOSTOn429WithoutIdempotencyKey(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddJSON(429, map[string]any{"message": "slow down"}).
		AddJSON(201, map[string]any{"id": "job-1"})

	if _, err := tc.Client.Jobs().Create(context.Background(), map[string]any{"conversion": []any{}}); err != nil {
		t.Fatal(err)
	}
	if tc.HTTP.Count() != 2 {
		t.Fatalf("429 is safe to retry for any method: %d attempts", tc.HTTP.Count())
	}
}

func TestNeverRetriesNonReplayableStreamOn429(t *testing.T) {
	tc := testutil.NewTestClient(t)
	job := api2convert.JobFromMap(map[string]any{
		"id": "job-9", "token": "tok", "server": "https://up/v2",
		"status": map[string]any{"code": "incomplete"},
	})
	tc.HTTP.AddJSON(429, map[string]any{"message": "slow down"})

	_, err := tc.Client.Jobs().Upload(context.Background(), job, strings.NewReader("data"))
	var rl *api2convert.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %T, want *RateLimitError", err)
	}
	if tc.HTTP.Count() != 1 {
		t.Fatalf("a one-shot stream body must not be retried: %d attempts", tc.HTTP.Count())
	}
	if tc.HTTP.At(0).Replayable {
		t.Fatal("io.Reader upload should be marked non-replayable")
	}
}

func TestRetriesReplayableBytesUploadOn429(t *testing.T) {
	tc := testutil.NewTestClient(t)
	job := api2convert.JobFromMap(map[string]any{
		"id": "job-9", "token": "tok", "server": "https://up/v2",
		"status": map[string]any{"code": "incomplete"},
	})
	tc.HTTP.
		AddJSON(429, map[string]any{"message": "slow down"}).
		AddJSON(200, map[string]any{"id": "in-1", "type": "upload"})

	if _, err := tc.Client.Jobs().Upload(context.Background(), job, []byte("data")); err != nil {
		t.Fatal(err)
	}
	if tc.HTTP.Count() != 2 {
		t.Fatalf("a replayable []byte upload should retry on 429: %d attempts", tc.HTTP.Count())
	}
}

func TestRetryAfterSecondsHonored(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddJSON(503, map[string]any{"message": "busy"}, hdr("Retry-After", "2")).
		AddJSON(200, map[string]any{"id": "j"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if d := tc.Sleeper.Durations(); len(d) != 1 || d[0] != 2*time.Second {
		t.Fatalf("Retry-After seconds not honored: %v", d)
	}
}

func TestRetryAfterHTTPDateHonored(t *testing.T) {
	tc := testutil.NewTestClient(t)
	when := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	tc.HTTP.
		AddJSON(503, map[string]any{"message": "busy"}, hdr("Retry-After", when)).
		AddJSON(200, map[string]any{"id": "j"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	d := tc.Sleeper.Durations()
	if len(d) != 1 || d[0] < 3*time.Second || d[0] > 6*time.Second {
		t.Fatalf("Retry-After HTTP-date not honored (~5s expected): %v", d)
	}
}

func TestRetryAfterClampedToCeiling(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.
		AddJSON(429, map[string]any{"message": "slow"}, hdr("Retry-After", "9999")).
		AddJSON(200, map[string]any{"id": "j"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if d := tc.Sleeper.Durations(); len(d) != 1 || d[0] != 120*time.Second {
		t.Fatalf("Retry-After must be clamped to 120s: %v", d)
	}
}
