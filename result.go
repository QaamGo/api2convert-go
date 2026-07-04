package api2convert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FileDownload is a downloadable output file. Returned by Client.Download. A
// download password supplied at construction is remembered and sent automatically
// on download.
type FileDownload struct {
	transport        *transport
	output           OutputFile
	downloadPassword *string
}

func newFileDownload(t *transport, output OutputFile, downloadPassword ...string) *FileDownload {
	var pw *string
	if len(downloadPassword) > 0 {
		p := downloadPassword[0]
		pw = &p
	}
	return &FileDownload{transport: t, output: output, downloadPassword: pw}
}

// URL returns the self-contained download URL (no auth required).
func (d *FileDownload) URL() string { return d.output.URI }

// Save streams the file to disk. pathOrDir is a file path, or a directory (the
// API filename is used, sanitized to a bare basename). A password set at
// conversion time is applied automatically; pass one here only to override it.
// Returns the path written to.
func (d *FileDownload) Save(ctx context.Context, pathOrDir string, downloadPassword ...string) (string, error) {
	target := d.resolveTarget(pathOrDir)
	parent := filepath.Dir(target)
	if parent == "" {
		parent = "."
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", newError("Could not create directory: "+parent, err)
	}

	resp, err := d.transport.openDownload(ctx, d.output.URI, d.headers(downloadPassword))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	f, err := os.Create(target)
	if err != nil {
		return "", newError("Could not write file: "+target, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, d.capReader(resp.Body)); err != nil {
		var a2c Api2ConvertError
		if errors.As(err, &a2c) {
			return "", err
		}
		return "", newError("Could not write file: "+target, err)
	}
	return target, nil
}

// Contents downloads the file and returns its contents (loads into memory).
func (d *FileDownload) Contents(ctx context.Context, downloadPassword ...string) ([]byte, error) {
	resp, err := d.transport.openDownload(ctx, d.output.URI, d.headers(downloadPassword))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(d.capReader(resp.Body))
	if err != nil {
		var a2c Api2ConvertError
		if errors.As(err, &a2c) {
			return nil, err
		}
		return nil, newError("failed to read download: "+err.Error(), err)
	}
	return data, nil
}

func (d *FileDownload) capReader(r io.Reader) io.Reader {
	if d.transport.config.maxDownloadBytes > 0 {
		return &capReader{r: r, limit: d.transport.config.maxDownloadBytes}
	}
	return r
}

func (d *FileDownload) resolveTarget(pathOrDir string) string {
	looksLikeDir := strings.HasSuffix(pathOrDir, "/") ||
		strings.HasSuffix(pathOrDir, `\`) ||
		isDirectory(pathOrDir)
	if looksLikeDir {
		name := firstUsable(safeName(d.output.Filename), safeName(d.output.ID), "output")
		return filepath.Join(strings.TrimRight(pathOrDir, `/\`), name)
	}
	return pathOrDir
}

func (d *FileDownload) headers(downloadPassword []string) map[string]string {
	if len(downloadPassword) > 0 {
		return map[string]string{"X-Oc-Download-Password": downloadPassword[0]}
	}
	if d.downloadPassword != nil {
		return map[string]string{"X-Oc-Download-Password": *d.downloadPassword}
	}
	return map[string]string{}
}

// ConversionResult is the result of a completed conversion. The common case is one
// output: result.Save(ctx, "out.pdf"). Jobs that produce several files expose them
// via Outputs and Download.
type ConversionResult struct {
	// Job is the completed job.
	Job              Job
	transport        *transport
	index            int
	downloadPassword *string
}

func newConversionResult(job Job, t *transport, index int, downloadPassword *string) *ConversionResult {
	return &ConversionResult{Job: job, transport: t, index: index, downloadPassword: downloadPassword}
}

// Output returns the selected output file (the first one by default). An index not
// present — including a negative one — is an error rather than wrapping around.
func (r *ConversionResult) Output() (OutputFile, error) {
	if r.index < 0 || r.index >= len(r.Job.Output) {
		return OutputFile{}, newError("The job produced no output files.", nil)
	}
	return r.Job.Output[r.index], nil
}

// Outputs returns all output files produced by the job.
func (r *ConversionResult) Outputs() []OutputFile { return r.Job.Output }

// URL returns the download URL of the selected output (self-contained, no auth).
func (r *ConversionResult) URL() (string, error) {
	o, err := r.Output()
	if err != nil {
		return "", err
	}
	return o.URI, nil
}

// Save downloads the selected output to disk. Returns the path written to.
func (r *ConversionResult) Save(ctx context.Context, pathOrDir string, downloadPassword ...string) (string, error) {
	dl, err := r.Download()
	if err != nil {
		return "", err
	}
	return dl.Save(ctx, pathOrDir, downloadPassword...)
}

// Contents downloads the selected output and returns its contents.
func (r *ConversionResult) Contents(ctx context.Context, downloadPassword ...string) ([]byte, error) {
	dl, err := r.Download()
	if err != nil {
		return nil, err
	}
	return dl.Contents(ctx, downloadPassword...)
}

// Download returns a FileDownload for a specific output (defaults to the selected
// one).
func (r *ConversionResult) Download(output ...OutputFile) (*FileDownload, error) {
	var out OutputFile
	if len(output) > 0 {
		out = output[0]
	} else {
		o, err := r.Output()
		if err != nil {
			return nil, err
		}
		out = o
	}
	if r.downloadPassword != nil {
		return newFileDownload(r.transport, out, *r.downloadPassword), nil
	}
	return newFileDownload(r.transport, out), nil
}

// capReader errors with a NetworkError once total bytes read exceed limit.
type capReader struct {
	r     io.Reader
	n     int64
	limit int64
}

func (c *capReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.limit {
		return n, &NetworkError{genericError{Message: fmt.Sprintf("download exceeded the configured maximum of %d bytes", c.limit)}}
	}
	return n, err
}

func isDirectory(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// safeName reduces an API-supplied name to a bare filename safe to append to a
// directory. A value like ../../etc/cron.d/evil (or one with separators or a NUL
// byte) must never escape the caller's chosen directory. Returns "" when nothing
// usable remains, so the caller can fall back.
func safeName(name string) string {
	if name == "" {
		return ""
	}
	cleaned := strings.ReplaceAll(name, "\x00", "")
	cleaned = strings.ReplaceAll(cleaned, `\`, "/")
	base := strings.TrimSpace(path.Base(cleaned))
	if base == "" || base == "." || base == ".." {
		return ""
	}
	return base
}

func firstUsable(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return "output"
}
