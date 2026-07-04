package api2convert_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go"
	"github.com/QaamGo/api2convert-go/internal/testutil"
)

func TestDefaultBaseURL(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "x"})
	if _, err := tc.Client.Jobs().Get(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.Last().URL; !strings.HasPrefix(url, api2convert.DefaultBaseURL+"/") {
		t.Fatalf("url = %q, want prefix %q", url, api2convert.DefaultBaseURL)
	}
}

func TestOptionOverridesBaseURLAndTrimsTrailingSlash(t *testing.T) {
	tc := testutil.NewTestClient(api2convert.WithBaseURL("https://example.test/v9/"))
	tc.HTTP.AddJSON(200, map[string]any{"id": "x"})
	if _, err := tc.Client.Jobs().Get(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if got, want := tc.HTTP.Last().URL, "https://example.test/v9/jobs/x"; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestAPIKeyFromEnvWhenEmpty(t *testing.T) {
	t.Setenv("API2CONVERT_API_KEY", "env-key")
	fake := &testutil.FakeSender{}
	c, err := api2convert.New("", api2convert.WithHTTPSender(fake))
	if err != nil {
		t.Fatalf("New with env key: %v", err)
	}
	fake.AddJSON(200, map[string]any{"id": "x"})
	if _, err := c.Jobs().Get(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if got := fake.Last().H("X-Oc-Api-Key"); got != "env-key" {
		t.Fatalf("auth header = %q, want env-key", got)
	}
}

func TestNoKeyReturnsConfigError(t *testing.T) {
	t.Setenv("API2CONVERT_API_KEY", "")
	_, err := api2convert.New("")
	var ce *api2convert.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T (%v), want *ConfigError", err, err)
	}
}
