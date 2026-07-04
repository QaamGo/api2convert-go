package api2convert

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// JobsResource gives full control over the job lifecycle. Most users only need
// Client.Convert, which is built on these methods. Methods are thin: build the
// request, call the transport, hydrate a model.
type JobsResource struct {
	transport *transport
	uploader  *fileUploader
}

// Create creates a job. Pass {"process": false} to stage it for uploads, then
// Start it once inputs are attached. An optional idempotencyKey makes the create
// retry-safe (sent as the Idempotency-Key header).
func (r *JobsResource) Create(ctx context.Context, payload map[string]any, idempotencyKey ...string) (*Job, error) {
	var headers map[string]string
	if len(idempotencyKey) > 0 && idempotencyKey[0] != "" {
		headers = map[string]string{"Idempotency-Key": idempotencyKey[0]}
	}
	res, err := r.transport.request(ctx, "POST", "/jobs", payload, nil, headers)
	if err != nil {
		return nil, err
	}
	job := JobFromMap(asObject(res))
	return &job, nil
}

// Get fetches a job by id.
func (r *JobsResource) Get(ctx context.Context, jobID string) (*Job, error) {
	res, err := r.transport.request(ctx, "GET", "/jobs/"+jobID, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	job := JobFromMap(asObject(res))
	return &job, nil
}

// List lists the current key's jobs (paginated, 50 per page). An empty status
// lists all; page <= 0 defaults to 1.
func (r *JobsResource) List(ctx context.Context, status string, page int) ([]Job, error) {
	if page <= 0 {
		page = 1
	}
	query := url.Values{"page": {strconv.Itoa(page)}}
	if status != "" {
		query.Set("status", status)
	}
	res, err := r.transport.request(ctx, "GET", "/jobs", nil, query, nil)
	if err != nil {
		return nil, err
	}
	return mapObjects(res, JobFromMap), nil
}

// Update patches a job (e.g. {"process": true} to start it).
func (r *JobsResource) Update(ctx context.Context, jobID string, payload map[string]any) (*Job, error) {
	res, err := r.transport.request(ctx, "PATCH", "/jobs/"+jobID, payload, nil, nil)
	if err != nil {
		return nil, err
	}
	job := JobFromMap(asObject(res))
	return &job, nil
}

// Start begins processing a staged job (process: true).
func (r *JobsResource) Start(ctx context.Context, jobID string) (*Job, error) {
	return r.Update(ctx, jobID, map[string]any{"process": true})
}

// Cancel cancels a job (whether staged or processing).
func (r *JobsResource) Cancel(ctx context.Context, jobID string) error {
	_, err := r.transport.request(ctx, "DELETE", "/jobs/"+jobID, nil, nil, nil)
	return err
}

// AddInput attaches an input by descriptor, e.g. a remote URL:
// AddInput(ctx, jobID, map[string]any{"type": "remote", "source": "https://..."}).
func (r *JobsResource) AddInput(ctx context.Context, jobID string, descriptor map[string]any) (*InputFile, error) {
	res, err := r.transport.request(ctx, "POST", "/jobs/"+jobID+"/input", descriptor, nil, nil)
	if err != nil {
		return nil, err
	}
	in := InputFileFromMap(asObject(res))
	return &in, nil
}

// Upload uploads a local file (path string, []byte or io.Reader) to the job's
// upload server. An optional filename overrides the advertised name.
func (r *JobsResource) Upload(ctx context.Context, job Job, file any, filename ...string) (*InputFile, error) {
	return r.uploader.upload(ctx, job, file, filename...)
}

// Outputs returns the outputs produced by the job (use Get or Wait first).
func (r *JobsResource) Outputs(ctx context.Context, jobID string) ([]OutputFile, error) {
	res, err := r.transport.request(ctx, "GET", "/jobs/"+jobID+"/output", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return mapObjects(res, OutputFileFromMap), nil
}

// Wait polls with backoff until the job reaches a terminal status. It returns a
// *ConversionFailedError on a failed/canceled job (unless throwOnFailure is
// false) and a *ConversionTimeoutError past the deadline. A timeout <= 0 uses the
// configured default. The interval is floored and the total wait capped, so no
// configuration can busy-loop or poll unbounded.
func (r *JobsResource) Wait(ctx context.Context, jobID string, timeout time.Duration, throwOnFailure bool) (*Job, error) {
	cfg := r.transport.config
	if timeout <= 0 {
		timeout = cfg.pollTimeout
	}
	if timeout > MaxPollTimeout {
		timeout = MaxPollTimeout
	}
	maxInterval := cfg.pollMaxInterval
	if maxInterval < MinPollInterval {
		maxInterval = MinPollInterval
	}
	interval := cfg.pollInterval
	if interval < MinPollInterval {
		interval = MinPollInterval
	}
	deadline := time.Now().Add(timeout)

	for {
		job, err := r.Get(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if (job.IsFailed() || job.IsCanceled()) && throwOnFailure {
			return nil, newConversionFailedError(*job)
		}
		if job.IsTerminal() {
			return job, nil
		}
		if !time.Now().Before(deadline) {
			return nil, newConversionTimeoutError(*job, timeout.Seconds())
		}
		if err := r.transport.pause(ctx, interval.Seconds()); err != nil {
			return nil, err
		}
		interval = time.Duration(float64(interval) * 1.5)
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

// ConversionsResource is the conversions catalog (GET /conversions) — the source
// of truth for which targets exist and which options each accepts.
type ConversionsResource struct {
	transport *transport
}

// List lists supported conversions, optionally filtered by category/target. Each
// entry is a map: {id, category, target, options}. An empty category/target is
// omitted; page <= 0 defaults to 1.
func (r *ConversionsResource) List(ctx context.Context, category, target string, page int) ([]map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	query := url.Values{"page": {strconv.Itoa(page)}}
	if category != "" {
		query.Set("category", category)
	}
	if target != "" {
		query.Set("target", target)
	}
	res, err := r.transport.request(ctx, "GET", "/conversions", nil, query, nil)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, item := range asList(res) {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// Options returns the option schema (type / enum / default / range) for a single
// target. An optional category disambiguates an ambiguous target.
func (r *ConversionsResource) Options(ctx context.Context, target string, category ...string) (map[string]any, error) {
	cat := ""
	if len(category) > 0 {
		cat = category[0]
	}
	rows, err := r.List(ctx, cat, target, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{}, nil
	}
	return asObject(rows[0]["options"]), nil
}

// PresetsResource manages saved conversion presets (reusable named target +
// options).
type PresetsResource struct {
	transport *transport
}

// List lists presets, optionally filtered by category / target / filter (empty
// values are omitted).
func (r *PresetsResource) List(ctx context.Context, category, target, filter string) ([]Preset, error) {
	query := url.Values{}
	if category != "" {
		query.Set("category", category)
	}
	if target != "" {
		query.Set("target", target)
	}
	if filter != "" {
		query.Set("filter", filter)
	}
	res, err := r.transport.request(ctx, "GET", "/presets", nil, query, nil)
	if err != nil {
		return nil, err
	}
	return mapObjects(res, PresetFromMap), nil
}

// Create creates a preset from {name, target, options, scope?, category?}.
func (r *PresetsResource) Create(ctx context.Context, payload map[string]any) (*Preset, error) {
	res, err := r.transport.request(ctx, "POST", "/presets", payload, nil, nil)
	if err != nil {
		return nil, err
	}
	p := PresetFromMap(asObject(res))
	return &p, nil
}

// Get fetches a preset by id.
func (r *PresetsResource) Get(ctx context.Context, presetID string) (*Preset, error) {
	res, err := r.transport.request(ctx, "GET", "/presets/"+presetID, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	p := PresetFromMap(asObject(res))
	return &p, nil
}

// Update patches a preset.
func (r *PresetsResource) Update(ctx context.Context, presetID string, payload map[string]any) (*Preset, error) {
	res, err := r.transport.request(ctx, "PATCH", "/presets/"+presetID, payload, nil, nil)
	if err != nil {
		return nil, err
	}
	p := PresetFromMap(asObject(res))
	return &p, nil
}

// Delete deletes a preset.
func (r *PresetsResource) Delete(ctx context.Context, presetID string) error {
	_, err := r.transport.request(ctx, "DELETE", "/presets/"+presetID, nil, nil, nil)
	return err
}

// StatsResource returns API usage statistics. The response shape is free-form
// (returned as-is). filter is either an API key to scope to, or "all".
type StatsResource struct {
	transport *transport
}

// Day returns usage for a day (format yyyy-mm-dd).
func (r *StatsResource) Day(ctx context.Context, day, filter string) (any, error) {
	return r.transport.request(ctx, "GET", "/stats/day/"+day+"/"+statsFilter(filter), nil, nil, nil)
}

// Month returns usage for a month (format yyyy-mm).
func (r *StatsResource) Month(ctx context.Context, month, filter string) (any, error) {
	return r.transport.request(ctx, "GET", "/stats/month/"+month+"/"+statsFilter(filter), nil, nil, nil)
}

// Year returns usage for a year (format yyyy).
func (r *StatsResource) Year(ctx context.Context, year, filter string) (any, error) {
	return r.transport.request(ctx, "GET", "/stats/year/"+year+"/"+statsFilter(filter), nil, nil, nil)
}

func statsFilter(filter string) string {
	if filter == "" {
		return "all"
	}
	return filter
}

// ContractsResource returns information about the account's active contracts
// (free-form response).
type ContractsResource struct {
	transport *transport
}

// Get returns the account's contract information.
func (r *ContractsResource) Get(ctx context.Context) (any, error) {
	return r.transport.request(ctx, "GET", "/contracts", nil, nil, nil)
}
