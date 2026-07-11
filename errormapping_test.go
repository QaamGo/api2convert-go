package api2convert_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func TestStatusMapsToTypedError(t *testing.T) {
	// maxRetries=0 so 429/5xx surface immediately (one attempt each).
	tc := testutil.NewTestClient(t, api2convert.WithMaxRetries(0))

	cases := []struct {
		status int
		check  func(err error) bool
		name   string
	}{
		{400, func(e error) bool { var t *api2convert.ValidationError; return errors.As(e, &t) }, "ValidationError"},
		{401, func(e error) bool { var t *api2convert.AuthenticationError; return errors.As(e, &t) }, "AuthenticationError"},
		{402, func(e error) bool { var t *api2convert.PaymentRequiredError; return errors.As(e, &t) }, "PaymentRequiredError"},
		{403, func(e error) bool { var t *api2convert.AuthenticationError; return errors.As(e, &t) }, "AuthenticationError"},
		{404, func(e error) bool { var t *api2convert.NotFoundError; return errors.As(e, &t) }, "NotFoundError"},
		{422, func(e error) bool { var t *api2convert.ValidationError; return errors.As(e, &t) }, "ValidationError"},
		{429, func(e error) bool { var t *api2convert.RateLimitError; return errors.As(e, &t) }, "RateLimitError"},
		{418, func(e error) bool { var t *api2convert.APIError; return errors.As(e, &t) }, "APIError"},
		{500, func(e error) bool { var t *api2convert.ServerError; return errors.As(e, &t) }, "ServerError"},
		{503, func(e error) bool { var t *api2convert.ServerError; return errors.As(e, &t) }, "ServerError"},
	}
	for _, c := range cases {
		tc.HTTP.AddJSON(c.status, map[string]any{"message": "boom"}, http.Header{"X-Request-Id": []string{"req-42"}})
		_, err := tc.Client.Jobs().Get(context.Background(), "j")
		if err == nil || !c.check(err) {
			t.Fatalf("status %d: err = %T (%v), want %s", c.status, err, err, c.name)
		}
		// Every HTTP error also satisfies the HTTPError interface with fields.
		var he api2convert.HTTPError
		if !errors.As(err, &he) {
			t.Fatalf("status %d: err does not satisfy HTTPError", c.status)
		}
		if he.Status() != c.status {
			t.Fatalf("status %d: HTTPError.Status() = %d", c.status, he.Status())
		}
		if he.RequestID() != "req-42" {
			t.Fatalf("status %d: RequestID = %q", c.status, he.RequestID())
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("status %d: message not extracted from body: %q", c.status, err.Error())
		}
		if strings.Contains(err.Error(), "test-key") {
			t.Fatalf("status %d: API key leaked into error message", c.status)
		}
	}
}

func TestRateLimitErrorCarriesRetryAfter(t *testing.T) {
	tc := testutil.NewTestClient(t, api2convert.WithMaxRetries(0))
	tc.HTTP.AddJSON(429, map[string]any{"message": "slow"}, http.Header{"Retry-After": []string{"30"}})

	_, err := tc.Client.Jobs().Get(context.Background(), "j")
	var rl *api2convert.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %T, want *RateLimitError", err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 30 {
		t.Fatalf("RetryAfter = %v, want 30", rl.RetryAfter)
	}
}

func TestNonJSONSuccessBodyIsNetworkError(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddText(200, "<html>not json</html>")

	_, err := tc.Client.Jobs().Get(context.Background(), "j")
	var ne *api2convert.NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T, want *NetworkError", err)
	}
}
