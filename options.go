package api2convert

import (
	"strings"
	"time"
)

// Option configures the client at construction. Pass Options to New.
type Option func(*clientBuilder)

type clientBuilder struct {
	baseURL          string
	timeout          time.Duration
	maxRetries       int
	pollInterval     time.Duration
	pollMaxInterval  time.Duration
	pollTimeout      time.Duration
	maxDownloadBytes int64
	sender           HttpSender
	sleeper          Sleeper
	rand             Rand
}

func newClientBuilder() *clientBuilder {
	return &clientBuilder{
		baseURL:         DefaultBaseURL,
		timeout:         30 * time.Second,
		maxRetries:      2,
		pollInterval:    1 * time.Second,
		pollMaxInterval: 5 * time.Second,
		pollTimeout:     300 * time.Second,
	}
}

// buildConfig clamps every knob so a caller value can neither busy-loop the poll
// (interval floor) nor poll unbounded (timeout ceiling).
func (b *clientBuilder) buildConfig(apiKey string) config {
	pollInterval := b.pollInterval
	if pollInterval < MinPollInterval {
		pollInterval = MinPollInterval
	}
	pollMaxInterval := b.pollMaxInterval
	if pollMaxInterval < pollInterval {
		pollMaxInterval = pollInterval
	}
	pollTimeout := b.pollTimeout
	if pollTimeout < 0 {
		pollTimeout = 0
	}
	if pollTimeout > MaxPollTimeout {
		pollTimeout = MaxPollTimeout
	}
	timeout := b.timeout
	if timeout < 1*time.Second {
		timeout = 1 * time.Second
	}
	maxRetries := b.maxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	maxDownloadBytes := b.maxDownloadBytes
	if maxDownloadBytes < 0 {
		maxDownloadBytes = 0
	}
	return config{
		apiKey:           apiKey,
		baseURL:          strings.TrimRight(b.baseURL, "/"),
		timeout:          timeout,
		maxRetries:       maxRetries,
		pollInterval:     pollInterval,
		pollMaxInterval:  pollMaxInterval,
		pollTimeout:      pollTimeout,
		maxDownloadBytes: maxDownloadBytes,
	}
}

// WithBaseURL sets the API base URL (default https://api.api2convert.com/v2).
func WithBaseURL(u string) Option { return func(b *clientBuilder) { b.baseURL = u } }

// WithTimeout sets the per-request network timeout (default 30s, min 1s).
func WithTimeout(d time.Duration) Option { return func(b *clientBuilder) { b.timeout = d } }

// WithMaxRetries sets the number of automatic retries for transient failures
// (429 / 5xx / network) (default 2, min 0).
func WithMaxRetries(n int) Option { return func(b *clientBuilder) { b.maxRetries = n } }

// WithPollInterval sets the first poll interval when waiting for a job (default
// 1s, floored to 500ms).
func WithPollInterval(d time.Duration) Option { return func(b *clientBuilder) { b.pollInterval = d } }

// WithPollMaxInterval sets the upper bound the poll interval backs off to
// (default 5s).
func WithPollMaxInterval(d time.Duration) Option {
	return func(b *clientBuilder) { b.pollMaxInterval = d }
}

// WithPollTimeout sets how long to wait for a job before giving up (default 300s,
// capped at 14400s).
func WithPollTimeout(d time.Duration) Option { return func(b *clientBuilder) { b.pollTimeout = d } }

// WithMaxDownloadBytes caps the size of a downloaded file; a larger response
// yields a NetworkError instead of an unbounded read. 0 (the default) means
// unlimited. This is an additive Go-only hardening beyond the shared contract.
func WithMaxDownloadBytes(n int64) Option { return func(b *clientBuilder) { b.maxDownloadBytes = n } }

// WithHTTPSender brings your own HTTP transport (defaults to a net/http sender).
// Primarily a test seam.
func WithHTTPSender(s HttpSender) Option { return func(b *clientBuilder) { b.sender = s } }

// WithSleeper injects the delay function used by retry/poll backoff (handy in
// tests).
func WithSleeper(s Sleeper) Option { return func(b *clientBuilder) { b.sleeper = s } }

// WithRand injects a [0,1) random source for backoff jitter (handy in tests).
func WithRand(r Rand) Option { return func(b *clientBuilder) { b.rand = r } }

// ConvertOption is an extra control for Convert / ConvertAsync. These named
// controls are kept strictly separate from the open-ended conversion options map
// (see WithConversionOptions), so open-ended API option keys can never collide
// with SDK control keys.
type ConvertOption func(*convertParams)

type convertParams struct {
	options          map[string]any
	category         *string
	callback         *string
	filename         *string
	downloadPassword *string
	outputIndex      int
	timeout          *time.Duration
	outputTargets    []OutputTarget
}

func newConvertParams(opts []ConvertOption) *convertParams {
	p := &convertParams{}
	for _, o := range opts {
		o(p)
	}
	return p
}

// WithConversionOptions sets the target-specific conversion options, passed 1:1 to
// the API's conversion "options". Discover valid options via Client.Options.
func WithConversionOptions(o map[string]any) ConvertOption {
	return func(p *convertParams) { p.options = o }
}

// WithCategory disambiguates an ambiguous target format.
func WithCategory(c string) ConvertOption { return func(p *convertParams) { p.category = &c } }

// WithCallback sets a webhook URL to notify on status change (sets notify_status:
// true). Applies to ConvertAsync only.
func WithCallback(u string) ConvertOption { return func(p *convertParams) { p.callback = &u } }

// WithFilename sets the advertised filename for an uploaded local file / stream.
func WithFilename(f string) ConvertOption { return func(p *convertParams) { p.filename = &f } }

// WithDownloadPassword protects every output; it is remembered on the returned
// result and sent automatically on download.
func WithDownloadPassword(pw string) ConvertOption {
	return func(p *convertParams) { p.downloadPassword = &pw }
}

// WithOutputIndex selects which output file the result selects (default 0).
// Applies to Convert only; ConvertAsync returns the Job (no result to select
// from) and ignores it.
func WithOutputIndex(i int) ConvertOption { return func(p *convertParams) { p.outputIndex = i } }

// WithConvertTimeout overrides the poll timeout for this conversion. Applies to
// Convert only (which waits); ConvertAsync does not poll and ignores it.
func WithConvertTimeout(d time.Duration) ConvertOption {
	return func(p *convertParams) { p.timeout = &d }
}

// WithOutputTarget attaches a cloud delivery target to the conversion's
// output_target (never merged into the conversion options map). Repeatable —
// each call appends. When any output target is set the conversion delivers to
// your storage and produces no local output, so Convert returns the completed
// job without downloading.
func WithOutputTarget(target OutputTarget) ConvertOption {
	return func(p *convertParams) { p.outputTargets = append(p.outputTargets, target) }
}

// WithOutputTargets attaches several cloud delivery targets at once (see
// WithOutputTarget). Appends to any already set.
func WithOutputTargets(targets ...OutputTarget) ConvertOption {
	return func(p *convertParams) { p.outputTargets = append(p.outputTargets, targets...) }
}
