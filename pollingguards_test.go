package api2convert_test

import (
	"context"
	"errors"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func TestWaitPollsAtLeastOnceEvenWithTinyTimeout(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "processing"}})

	_, err := tc.Client.Jobs().Wait(context.Background(), "j", time.Nanosecond, true)
	var to *api2convert.ConversionTimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %T, want *ConversionTimeoutError", err)
	}
	if tc.HTTP.Count() != 1 {
		t.Fatalf("expected exactly one poll, got %d", tc.HTTP.Count())
	}
}

func TestPollIntervalFlooredToMinimum(t *testing.T) {
	// A configured interval below the floor is clamped so polling can't busy-spin.
	tc := testutil.NewTestClient(api2convert.WithPollInterval(1 * time.Millisecond))
	tc.HTTP.
		AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "processing"}}).
		AddJSON(200, map[string]any{"id": "j", "status": map[string]any{"code": "completed"}})

	if _, err := tc.Client.Jobs().Wait(context.Background(), "j", 0, true); err != nil {
		t.Fatal(err)
	}
	d := tc.Sleeper.Durations()
	if len(d) != 1 || d[0] < api2convert.MinPollInterval {
		t.Fatalf("poll interval not floored: %v (min %v)", d, api2convert.MinPollInterval)
	}
}
