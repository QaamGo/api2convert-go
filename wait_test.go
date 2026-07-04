package api2convert_test

import (
	"context"
	"errors"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go"
	"github.com/QaamGo/api2convert-go/internal/testutil"
)

func TestWaitPollsUntilCompleted(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "processing"}}).
		AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "processing"}}).
		AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "completed"}})

	job, err := tc.Client.Jobs().Wait(context.Background(), "j", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !job.IsCompleted() {
		t.Fatalf("status = %q", job.Status.Code)
	}
	if tc.HTTP.Count() != 3 {
		t.Fatalf("polled %d times, want 3", tc.HTTP.Count())
	}
	// Two pauses between three polls; jitter disabled (rng==0), so exact values:
	// pollInterval (1s) then interval*1.5 (1.5s).
	got := tc.Sleeper.Durations()
	if len(got) != 2 || got[0] != 1*time.Second || got[1] != 1500*time.Millisecond {
		t.Fatalf("sleep durations = %v, want [1s 1.5s]", got)
	}
}

func TestWaitReturnsConversionFailedError(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{
		"id":     "j",
		"status": map[string]any{"code": "failed"},
		"errors": []any{map[string]any{"code": 7, "message": "bad input"}},
	})

	_, err := tc.Client.Jobs().Wait(context.Background(), "j", 0, true)
	var cf *api2convert.ConversionFailedError
	if !errors.As(err, &cf) {
		t.Fatalf("err = %T (%v), want *ConversionFailedError", err, err)
	}
	if cf.Job.ID != "j" || len(cf.Errors()) != 1 || cf.Errors()[0].Message != "bad input" {
		t.Fatalf("attached job/errors wrong: %+v", cf)
	}
}

func TestWaitThrowOnFailureFalseReturnsJob(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "failed"}})

	job, err := tc.Client.Jobs().Wait(context.Background(), "j", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !job.IsFailed() {
		t.Fatalf("status = %q", job.Status.Code)
	}
}

func TestWaitTimesOut(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "processing"}})

	_, err := tc.Client.Jobs().Wait(context.Background(), "j", time.Nanosecond, true)
	var to *api2convert.ConversionTimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %T (%v), want *ConversionTimeoutError", err, err)
	}
	if to.Job.Status.Code != "processing" {
		t.Fatalf("timeout job status = %q", to.Job.Status.Code)
	}
}
