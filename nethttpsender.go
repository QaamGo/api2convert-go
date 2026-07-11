package api2convert

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"
)

// netHTTPSender is the default HTTPSender, backed by net/http (no third-party
// dependency).
//
// Redirect policy is client-level in net/http, so this holds two clients sharing
// one *http.Transport:
//
//   - noRedirect never follows a redirect (CheckRedirect returns
//     http.ErrUseLastResponse). It is used for every authenticated request. The
//     account key / per-job token / download password ride in custom X-Oc-*
//     headers, and net/http's default redirect handler forwards custom headers
//     across a cross-host redirect (since Go 1.8 it strips only
//     Authorization/Www-Authenticate/Cookie/Cookie2 on a domain change), so a
//     redirect-following client would leak the secret to another host.
//   - follow follows normal redirects; it is used only for the self-contained,
//     no-auth download path, where storage/CDN URLs legitimately redirect.
//
// The choice is made per request from Request.FollowRedirects.
type netHTTPSender struct {
	noRedirect *http.Client
	follow     *http.Client
}

func newNetHTTPSender(timeout time.Duration) *netHTTPSender {
	// Clone the default transport when possible; fall back to a fresh one if a host
	// app replaced http.DefaultTransport with a different RoundTripper (a bare type
	// assertion would panic).
	var base *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		base = dt.Clone()
	} else {
		base = &http.Transport{}
	}
	// Bound the pre-body phase (TLS handshake + waiting for the response headers) so
	// a stalled server can't hang forever — without capping a long but healthy body
	// transfer, which the caller's context governs. Streamed requests rely on these
	// budgets; JSON requests additionally get a whole-exchange deadline (see Send).
	if timeout > 0 {
		base.TLSHandshakeTimeout = timeout
		base.ResponseHeaderTimeout = timeout
	}
	return &netHTTPSender{
		noRedirect: &http.Client{
			Transport: base,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		follow: &http.Client{Transport: base},
	}
}

func (s *netHTTPSender) Send(ctx context.Context, req *Request) (*Response, error) {
	// Validate the URL up front so a malformed API-supplied URI surfaces as a
	// (non-retryable) NetworkError, not a raw error from URL parsing.
	if _, err := url.Parse(req.URL); err != nil {
		return nil, &NetworkError{genericError{Message: "Invalid request URL: " + req.URL, Cause: err}}
	}

	var body io.Reader
	if req.MakeBody != nil {
		b, err := req.MakeBody()
		if err != nil {
			return nil, err
		}
		body = b
	} else if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	// A non-streamed (JSON control-plane) request gets a whole-exchange deadline. A
	// streamed request must not — its body transfer is bounded only by the caller's
	// context; the pre-body phase is bounded by the transport timeouts set in
	// newNetHTTPSender.
	rctx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 && !req.Stream {
		rctx, cancel = context.WithTimeout(ctx, req.Timeout)
	}

	httpReq, err := http.NewRequestWithContext(rctx, req.Method, req.URL, body)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, &NetworkError{genericError{Message: "Invalid request URL: " + req.URL, Cause: err}}
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := s.noRedirect
	if req.FollowRedirects {
		client = s.follow
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		// A genuine transport failure (DNS/connection/TLS/timeout) is returned as
		// a plain error so the transport treats it as transient and may retry it.
		return nil, err
	}

	return &Response{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Header:     resp.Header,
		Body:       &cancelBody{ReadCloser: resp.Body, cancel: cancel},
	}, nil
}

// cancelBody releases the per-request timeout context when the body is closed.
type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelBody) Close() error {
	err := c.ReadCloser.Close()
	if c.cancel != nil {
		c.cancel()
	}
	return err
}
