package api2convert

import (
	"context"
	"io"
	"net/http"
	"time"
)

// The pluggable HTTP transport seam.
//
// The transport builds a Request and hands it to an HttpSender; the default
// sender (netHTTPSender) runs it over net/http. Tests inject a fake sender to
// assert on requests and return canned responses. Mirrors the Java HttpSender /
// Node httpSender / Python httpx injection / PHP PSR-18 seams.

// Request is a transport-agnostic HTTP request.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string

	// Body is a materialized, replayable body (e.g. JSON bytes). Ignored when
	// MakeBody is set.
	Body []byte

	// MakeBody produces a fresh body reader per attempt (for streamed / multipart
	// requests) so a replay re-creates it. Takes precedence over Body. May be nil.
	// If the returned reader is an io.Closer it is closed after the body is sent.
	MakeBody func() (io.Reader, error)

	// FollowRedirects: only a no-secret download opts in. Any request carrying an
	// X-Oc-* secret header must keep this false so a redirect cannot forward the
	// secret to another host.
	FollowRedirects bool

	// Replayable reports whether the body can be re-sent on a retry (false for
	// one-shot streams).
	Replayable bool

	// Timeout is the per-request network timeout.
	Timeout time.Duration
}

// Response is a transport-agnostic HTTP response. Body is a single-use stream the
// caller must close.
type Response struct {
	Status     int
	StatusText string
	Header     http.Header
	Body       io.ReadCloser
}

// HeaderGet returns the named response header (case-insensitive), or "".
func (r *Response) HeaderGet(name string) string {
	if r.Header == nil {
		return ""
	}
	return r.Header.Get(name)
}

// HttpSender is the pluggable transport. The default is netHTTPSender; tests
// inject a fake. Send must respect ctx for cancellation.
type HttpSender interface {
	Send(ctx context.Context, req *Request) (*Response, error)
}

// Sleeper delays for d, returning early with ctx.Err() if ctx is canceled.
// Injectable; the real implementation uses a timer.
type Sleeper func(ctx context.Context, d time.Duration) error

// Rand returns a [0,1) float for backoff jitter. Injectable for deterministic
// tests.
type Rand func() float64

// defaultSleeper sleeps for d, honoring ctx cancellation.
func defaultSleeper(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
