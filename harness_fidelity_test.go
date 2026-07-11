package api2convert_test

import (
	"context"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// TestRequestRecordsConfiguredTimeout pins that the per-request timeout is
// actually carried on the outbound request (M3d): before the harness recorded
// Timeout, removing it from a call site passed the whole suite.
func TestRequestRecordsConfiguredTimeout(t *testing.T) {
	tc := testutil.NewTestClient(t, api2convert.WithTimeout(12*time.Second))
	tc.HTTP.AddJSON(200, map[string]any{"id": "x"})

	if _, err := tc.Client.Jobs().Get(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if got := tc.HTTP.Last().Timeout; got != 12*time.Second {
		t.Fatalf("recorded request timeout = %v, want 12s", got)
	}
}

// TestDownloadRequestIsMarkedStreaming pins that downloads flow through the
// streaming path (Stream=true) so the whole-exchange timeout does not cap them
// (H2 regression guard, via the now-recorded Stream flag).
func TestDownloadRequestIsMarkedStreaming(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("data"))

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	if _, err := tc.Client.Download(out).Contents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !tc.HTTP.Last().Stream {
		t.Fatal("a download request must be marked streaming (Stream=true)")
	}
}

// TestWaitHonorsContextCancellation proves a canceled context stops a poll wait
// (M3c): before the fake honored ctx and the sleeper checked it, a regression that
// ignored cancellation would have passed every offline test.
func TestWaitHonorsContextCancellation(t *testing.T) {
	tc := testutil.NewTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the first poll

	if _, err := tc.Client.Jobs().Wait(ctx, "j", 0, true); err == nil {
		t.Fatal("Wait must return an error when the context is canceled")
	}
}
