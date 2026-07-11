package api2convert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Public configuration constants.
const (
	// DefaultBaseURL is the default API base URL (includes the /v2 path segment).
	DefaultBaseURL = "https://api.api2convert.com/v2"
	// MinPollInterval is the hard floor for the job-poll interval; prevents a
	// busy-spin self-DDOS.
	MinPollInterval = 500 * time.Millisecond
	// MaxPollTimeout is the hard ceiling for the total job-poll timeout (4 hours);
	// bounds an unbounded poll.
	MaxPollTimeout = 14400 * time.Second
)

const (
	maxBackoff    = 8.0   // seconds
	maxRetryAfter = 120.0 // seconds; caps a hostile Retry-After so it can't stall for hours
)

var (
	retryableStatuses = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}
	idempotentMethods = map[string]bool{
		http.MethodGet: true, http.MethodHead: true, http.MethodPut: true,
		http.MethodDelete: true, http.MethodOptions: true, http.MethodTrace: true,
	}
	userAgent = "api2convert-go/" + Version + " " + runtime.Version()
)

// config is the frozen, clamped client configuration.
type config struct {
	apiKey           string
	baseURL          string
	timeout          time.Duration
	maxRetries       int
	pollInterval     time.Duration
	pollMaxInterval  time.Duration
	pollTimeout      time.Duration
	maxDownloadBytes int64 // 0 == unlimited
}

// transport is the HTTP layer: authenticated requests, transient-failure retries
// with jittered exponential backoff, error-response mapping to typed errors, and
// JSON decoding.
type transport struct {
	sender HttpSender
	config config
	sleep  Sleeper
	rand   Rand
}

// request performs an authenticated JSON request and returns the decoded body
// (a map[string]any, a []any, or an empty map for an empty body).
func (t *transport) request(ctx context.Context, method, path string, body any, query url.Values, headers map[string]string) (any, error) {
	reqHeaders := map[string]string{"X-Oc-Api-Key": t.config.apiKey}
	for k, v := range headers {
		reqHeaders[k] = v
	}
	var content []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, &NetworkError{genericError{Message: "failed to encode request body: " + err.Error(), Cause: err}}
		}
		content = b
		reqHeaders["Content-Type"] = "application/json"
	}
	req := &Request{
		Method:          method,
		URL:             t.url(path, query),
		Headers:         reqHeaders,
		Body:            content,
		FollowRedirects: false,
		Replayable:      true,
		Timeout:         t.config.timeout,
	}
	resp, err := t.send(ctx, req)
	if err != nil {
		return nil, err
	}
	return t.interpret(resp)
}

// send performs a request with retry/backoff. It adds the common Accept /
// User-Agent headers but no auth (callers add the header they need). A
// non-idempotent request is not replayed on a network/5xx error (the backend may
// have acted, so a blind retry could create a duplicate job); a non-replayable
// body is sent once.
func (t *transport) send(ctx context.Context, req *Request) (*Response, error) {
	if _, ok := req.Headers["Accept"]; !ok {
		req.Headers["Accept"] = "application/json"
	}
	req.Headers["User-Agent"] = userAgent
	idempotent := isIdempotent(req)
	attempt := 0

	for {
		resp, err := t.sender.Send(ctx, req)
		if err != nil {
			// Already-typed failures (e.g. a malformed URL) are terminal.
			var a2c Api2ConvertError
			if errors.As(err, &a2c) {
				return nil, err
			}
			if req.Replayable && idempotent && attempt < t.config.maxRetries {
				if berr := t.backoff(ctx, attempt, ""); berr != nil {
					return nil, berr
				}
				attempt++
				continue
			}
			return nil, &NetworkError{genericError{Message: "Request to API2Convert failed: " + t.redact(err.Error()), Cause: err}}
		}

		status := resp.Status
		mayRetry := retryableStatuses[status] &&
			req.Replayable &&
			attempt < t.config.maxRetries &&
			(status == 429 || idempotent)
		if mayRetry {
			discardBody(resp)
			if berr := t.backoff(ctx, attempt, resp.HeaderGet("Retry-After")); berr != nil {
				return nil, berr
			}
			attempt++
			continue
		}

		return resp, nil
	}
}

// interpret raises a typed error for error responses; otherwise decodes JSON.
func (t *transport) interpret(resp *Response) (any, error) {
	if err := t.ensureSuccessful(resp); err != nil {
		return nil, err
	}
	if resp.Status >= 300 {
		// A 3xx on the no-redirect authenticated JSON path is not a usable response:
		// an empty-body redirect would otherwise hydrate a zero-value model and make
		// Convert poll the wrong endpoint. Surface it as a typed error instead of
		// silently succeeding (mirrors openDownload's redirect guard).
		discardBody(resp)
		return nil, &NetworkError{genericError{Message: "API2Convert returned an unexpected redirect (HTTP " + strconv.Itoa(resp.Status) + ") on an authenticated request."}}
	}
	raw, err := readAllAndClose(resp.Body)
	if err != nil {
		return nil, &NetworkError{genericError{Message: "failed to read API2Convert response: " + t.redact(err.Error()), Cause: err}}
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	decoded, err := decodeJSON(raw)
	if err != nil {
		return nil, &NetworkError{genericError{Message: "API2Convert returned a non-JSON success response: " + t.redact(err.Error()), Cause: err}}
	}
	switch decoded.(type) {
	case map[string]any, []any:
		return decoded, nil
	default:
		return map[string]any{}, nil
	}
}

// ensureSuccessful returns the appropriate typed error when resp is an HTTP error
// (status >= 400). On success it returns nil without consuming the body.
func (t *transport) ensureSuccessful(resp *Response) error {
	status := resp.Status
	if status < 400 {
		return nil
	}
	body := decodeSafe(resp)
	message := "Request failed"
	if m, ok := body["message"].(string); ok && m != "" {
		message = m
	} else if resp.StatusText != "" {
		message = resp.StatusText
	}
	requestID := resp.HeaderGet("X-Request-Id")
	var retryAfter *int
	if status == 429 {
		retryAfter = parseRetryAfter(resp.HeaderGet("Retry-After"))
	}
	return newAPIError(status, message, requestID, body, retryAfter)
}

// openDownload opens a (self-contained) download URL and returns the response for
// streaming.
//
// A request carrying any X-Oc-* secret header (e.g. a download password) must not
// follow redirects; a plain, passwordless download may follow storage/CDN
// redirects. When a secret-bearing request is redirected, the no-redirect client
// returns the 3xx as-is — surfaced as a NetworkError so a silently-empty file
// never lands on disk.
func (t *transport) openDownload(ctx context.Context, uri string, headers map[string]string) (*Response, error) {
	carriesSecret := false
	for k := range headers {
		if strings.HasPrefix(strings.ToLower(k), "x-oc-") {
			carriesSecret = true
			break
		}
	}
	h := make(map[string]string, len(headers))
	for k, v := range headers {
		h[k] = v
	}
	h["Accept"] = "*/*" // a download is binary, not JSON
	req := &Request{
		Method:          http.MethodGet,
		URL:             uri,
		Headers:         h,
		FollowRedirects: !carriesSecret,
		Replayable:      true,
		Timeout:         t.config.timeout,
		Stream:          true,
	}
	resp, err := t.send(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := t.ensureSuccessful(resp); err != nil {
		return nil, err
	}
	if resp.Status >= 300 && resp.Status < 400 {
		discardBody(resp)
		return nil, &NetworkError{genericError{Message: "The download did not resolve: a redirect was not followed because the request carried a secret header."}}
	}
	if resp.Body == nil {
		// A conforming sender always sets Body; guard against a custom one that does
		// not, so Save/Contents never nil-panic on the deferred Close.
		return nil, &NetworkError{genericError{Message: "The download response had no body."}}
	}
	return resp, nil
}

// redact removes the account API key from a string before it is placed in an
// error message, upholding the guarantee that a key never surfaces in errors or
// logs — even if a wrapped transport error (e.g. a *url.Error echoing the URL)
// were to contain it.
func (t *transport) redact(s string) string {
	if t.config.apiKey == "" {
		return s
	}
	return strings.ReplaceAll(s, t.config.apiKey, "[REDACTED]")
}

func (t *transport) url(path string, query url.Values) string {
	u := t.config.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// pause sleeps for (at least) seconds with a small upward jitter (job polling).
func (t *transport) pause(ctx context.Context, seconds float64) error {
	return t.sleep(ctx, secondsToDuration(t.jitter(seconds)))
}

func (t *transport) backoff(ctx context.Context, attempt int, retryAfter string) error {
	ra := parseRetryAfter(retryAfter)
	var seconds float64
	if ra != nil && *ra > 0 {
		// Honor a positive Retry-After (capped). Not jittered: the server asked
		// for this exact delay.
		seconds = math.Min(maxRetryAfter, float64(*ra))
	} else {
		// A zero/past/absent Retry-After falls through to jittered exponential
		// backoff so we never retry-storm with no delay.
		seconds = t.jitter(math.Min(maxBackoff, 0.5*math.Pow(2, float64(attempt))))
	}
	return t.sleep(ctx, secondsToDuration(seconds))
}

// jitter adds a small upward jitter (0-25%) so correlated clients don't lockstep.
func (t *transport) jitter(seconds float64) float64 {
	return seconds + seconds*0.25*t.rand()
}

func isIdempotent(req *Request) bool {
	if idempotentMethods[strings.ToUpper(req.Method)] {
		return true
	}
	for name, value := range req.Headers {
		if strings.EqualFold(name, "Idempotency-Key") && value != "" {
			return true
		}
	}
	return false
}

// parseRetryAfter parses Retry-After (delay-seconds or HTTP-date) into whole
// seconds; never negative; nil when absent/unparseable.
func parseRetryAfter(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		n := int(math.Trunc(f))
		if n < 0 {
			n = 0
		}
		return &n
	}
	if tm, err := http.ParseTime(value); err == nil {
		n := int(math.Round(time.Until(tm).Seconds()))
		if n < 0 {
			n = 0
		}
		return &n
	}
	return nil
}

func secondsToDuration(seconds float64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func decodeJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// decodeSafe reads and closes the body, returning the decoded JSON object or an
// empty map (never errors).
func decodeSafe(resp *Response) map[string]any {
	raw, err := readAllAndClose(resp.Body)
	if err != nil || len(raw) == 0 {
		return map[string]any{}
	}
	decoded, err := decodeJSON(raw)
	if err != nil {
		return map[string]any{}
	}
	if m, ok := decoded.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// maxResponseBytes caps how much of a control-plane (API / error) JSON body the
// SDK buffers into memory, so a hostile or buggy server cannot force an unbounded
// read on these paths. File downloads are streamed and bounded separately by
// WithMaxDownloadBytes.
const maxResponseBytes = 16 << 20 // 16 MiB

func readAllAndClose(rc io.ReadCloser) ([]byte, error) {
	if rc == nil {
		return nil, nil
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxResponseBytes))
}

// discardBody drains and closes an unconsumed body to free the connection between
// retries.
func discardBody(resp *Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
}
