// Package api2convert is the official Go SDK for the API2Convert file-conversion
// API — convert, compress and transform files with one call.
//
// Quick start:
//
//	client, err := api2convert.New("YOUR_API_KEY")
//	if err != nil {
//		log.Fatal(err)
//	}
//	res, err := client.Convert(ctx, "invoice.docx", "pdf")
//	if err != nil {
//		log.Fatal(err)
//	}
//	if _, err := res.Save(ctx, "invoice.pdf"); err != nil {
//		log.Fatal(err)
//	}
//
// The API key falls back to the API2CONVERT_API_KEY environment variable when the
// empty string is passed. Convert hides the multi-step job lifecycle (create ->
// upload -> start -> poll -> download); for full control use client.Jobs() and the
// other resources.
package api2convert

import (
	"context"
	"os"
	"regexp"
)

var urlRE = regexp.MustCompile(`(?i)^https?://`)

// Client is the API2Convert client. Construct it with New. A Client is safe for
// concurrent use by multiple goroutines: its configuration is set once in New and
// never mutated, and the default HTTP sender and jitter source are themselves
// goroutine-safe. (If you inject a Rand via WithRand, it must also be safe for
// concurrent use.)
type Client struct {
	transport   *transport
	jobs        *JobsResource
	conversions *ConversionsResource
	presets     *PresetsResource
	stats       *StatsResource
	contracts   *ContractsResource
}

// New builds a client. apiKey falls back to the API2CONVERT_API_KEY environment
// variable when empty; New returns a *ConfigError if neither yields a key.
func New(apiKey string, opts ...Option) (*Client, error) {
	b := newClientBuilder()
	for _, o := range opts {
		o(b)
	}

	key := apiKey
	if key == "" {
		key = os.Getenv("API2CONVERT_API_KEY")
	}
	if key == "" {
		return nil, &ConfigError{genericError{Message: "No API key provided. Pass it to New or set the API2CONVERT_API_KEY environment variable."}}
	}

	cfg := b.buildConfig(key)
	sender := b.sender
	if sender == nil {
		sender = newNetHTTPSender(cfg.timeout)
	}
	sleeper := b.sleeper
	if sleeper == nil {
		sleeper = defaultSleeper
	}
	rnd := b.rand
	if rnd == nil {
		rnd = defaultRand
	}

	tr := &transport{sender: sender, config: cfg, sleep: sleeper, rand: rnd}
	up := &fileUploader{transport: tr}
	return &Client{
		transport:   tr,
		jobs:        &JobsResource{transport: tr, uploader: up},
		conversions: &ConversionsResource{transport: tr},
		presets:     &PresetsResource{transport: tr},
		stats:       &StatsResource{transport: tr},
		contracts:   &ContractsResource{transport: tr},
	}, nil
}

// Convert converts a file and waits for the result.
//
// input is a local file path, a public URL (^https?://), in-memory []byte, or an
// io.Reader. Name the target format in to, then Save the returned result. Extra
// controls (conversion options, category, download password, output index, poll
// timeout) are supplied via ConvertOption values (the With* functions).
func (c *Client) Convert(ctx context.Context, input any, to string, opts ...ConvertOption) (*ConversionResult, error) {
	p := newConvertParams(opts)
	job, err := c.startConversion(ctx, input, to, p, false)
	if err != nil {
		return nil, err
	}
	timeout := c.transport.config.pollTimeout
	if p.timeout != nil {
		timeout = *p.timeout
	}
	done, err := c.jobs.Wait(ctx, job.ID, timeout, true)
	if err != nil {
		return nil, err
	}
	return newConversionResult(*done, c.transport, p.outputIndex, p.downloadPassword), nil
}

// ConvertAsync starts a conversion without waiting. Pass WithCallback to be
// notified (sets notify_status), or poll later with client.Jobs().Get /
// client.Jobs().Wait.
func (c *Client) ConvertAsync(ctx context.Context, input any, to string, opts ...ConvertOption) (*Job, error) {
	p := newConvertParams(opts)
	return c.startConversion(ctx, input, to, p, true)
}

// Download returns a FileDownload for an output file. A downloadPassword is
// remembered and sent automatically on download (overridable per call). No I/O
// happens until Save / Contents is called.
func (c *Client) Download(output OutputFile, downloadPassword ...string) *FileDownload {
	return newFileDownload(c.transport, output, downloadPassword...)
}

// Options discovers the valid options (type / enum / default / range) for a
// target. An optional category disambiguates an ambiguous target.
func (c *Client) Options(ctx context.Context, target string, category ...string) (map[string]any, error) {
	return c.conversions.Options(ctx, target, category...)
}

// Jobs returns the jobs resource (full lifecycle control).
func (c *Client) Jobs() *JobsResource { return c.jobs }

// Conversions returns the conversions catalog resource.
func (c *Client) Conversions() *ConversionsResource { return c.conversions }

// Presets returns the presets resource.
func (c *Client) Presets() *PresetsResource { return c.presets }

// Stats returns the usage-statistics resource.
func (c *Client) Stats() *StatsResource { return c.stats }

// Contracts returns the contracts resource.
func (c *Client) Contracts() *ContractsResource { return c.contracts }

// Webhooks returns a webhook verifier — usable without a configured client.
func Webhooks() *WebhookVerifier { return &WebhookVerifier{} }

func (c *Client) startConversion(ctx context.Context, input any, to string, p *convertParams, isAsync bool) (*Job, error) {
	conversion := map[string]any{"target": to}
	if p.category != nil {
		conversion["category"] = *p.category
	}
	if len(p.options) > 0 {
		conversion["options"] = p.options
	}
	if len(p.outputTargets) > 0 {
		// Cloud delivery targets attach to the conversion's output_target and are
		// never merged into the options map, so open-ended API option keys can't
		// collide with them.
		targets := make([]any, 0, len(p.outputTargets))
		for _, t := range p.outputTargets {
			targets = append(targets, t.Descriptor())
		}
		conversion["output_target"] = targets
	}

	payload := map[string]any{"conversion": []any{conversion}}
	if isAsync && p.callback != nil {
		payload["callback"] = *p.callback
		payload["notify_status"] = true
	}
	if p.downloadPassword != nil {
		payload["download_passwords"] = []any{*p.downloadPassword}
	}

	// A cloud input imports the source straight from customer storage — a started
	// job with the descriptor inline, exactly like a remote URL (never staged /
	// uploaded).
	if ci, ok := input.(CloudInput); ok {
		payload["process"] = true
		payload["input"] = []any{ci.Descriptor()}
		return c.jobs.Create(ctx, payload)
	}

	if s, ok := input.(string); ok && urlRE.MatchString(s) {
		payload["process"] = true
		payload["input"] = []any{map[string]any{"type": "remote", "source": s}}
		return c.jobs.Create(ctx, payload)
	}

	payload["process"] = false
	created, err := c.jobs.Create(ctx, payload)
	if err != nil {
		return nil, err
	}
	if p.filename != nil {
		if _, err := c.jobs.Upload(ctx, *created, input, *p.filename); err != nil {
			return nil, err
		}
	} else {
		if _, err := c.jobs.Upload(ctx, *created, input); err != nil {
			return nil, err
		}
	}
	return c.jobs.Start(ctx, created.ID)
}

// defaultRand is the production jitter source (math/rand/v2 is concurrency-safe
// and needs no seeding).
func defaultRand() float64 { return randFloat64() }
