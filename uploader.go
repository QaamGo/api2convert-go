package api2convert

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

// fileUploader uploads a local file to a job's per-job upload server.
//
// This step is intentionally hand-written — it is NOT described by the OpenAPI
// spec. It posts a multipart/form-data body (field "file") to
// {job.Server}/upload-file/{job.ID} and authenticates with the per-job X-Api2convert-Token
// header — never the account API key. Bodies are streamed, so large files are not
// read into memory.
type fileUploader struct {
	transport *transport
}

func (u *fileUploader) upload(ctx context.Context, job Job, file any, filename ...string) (*InputFile, error) {
	if job.Server == "" || job.Token == "" {
		return nil, newError("Cannot upload: the job has no upload server/token. Create the job with process=false and upload before starting it.", nil)
	}

	uploadURL := strings.TrimRight(job.Server, "/") + "/upload-file/" + job.ID
	makeBody, contentType, replayable, err := u.buildBody(file, filename)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"X-Api2convert-Token": job.Token,
		"Content-Type":        contentType,
	}
	req := &Request{
		Method:          "POST",
		URL:             uploadURL,
		Headers:         headers,
		MakeBody:        makeBody,
		FollowRedirects: false,
		Replayable:      replayable,
		Timeout:         u.transport.config.timeout,
		Stream:          true,
	}

	resp, err := u.transport.send(ctx, req)
	if err != nil {
		return nil, err
	}
	decoded, err := u.transport.interpret(resp)
	if err != nil {
		return nil, err
	}
	in := InputFileFromMap(asObject(decoded))
	return &in, nil
}

func (u *fileUploader) buildBody(file any, filename []string) (makeBody func() (io.Reader, error), contentType string, replayable bool, err error) {
	// A single boundary is generated once and reused across attempts so the
	// Content-Type header always matches every regenerated body.
	boundary := randomBoundary()
	contentType = "multipart/form-data; boundary=" + boundary

	switch f := file.(type) {
	case string:
		info, statErr := os.Stat(f)
		if statErr != nil || info.IsDir() {
			return nil, "", false, newError("Input file not found: "+f, nil)
		}
		name := filenameOr(filename, filepath.Base(f))
		// Local file path — streamed via a fresh read stream per attempt (replayable).
		return func() (io.Reader, error) {
			handle, openErr := os.Open(f)
			if openErr != nil {
				return nil, newError("Input file not found: "+f, openErr)
			}
			return streamedMultipart(boundary, name, handle), nil
		}, contentType, true, nil

	case []byte:
		name := filenameOr(filename, "file")
		return func() (io.Reader, error) {
			return streamedMultipart(boundary, name, newBytesReadCloser(f)), nil
		}, contentType, true, nil

	case io.Reader:
		// One-shot streamed multipart (sent once; not replayed on a retry).
		name := filenameOr(filename, "file")
		return func() (io.Reader, error) {
			return streamedMultipart(boundary, name, f), nil
		}, contentType, false, nil

	default:
		return nil, "", false, newError("Unsupported upload input type.", nil)
	}
}

// filenameOr returns the provided filename (even when it is the empty string,
// mirroring the contract's "only unset falls back to the default"), else dflt.
func filenameOr(filename []string, dflt string) string {
	if len(filename) > 0 {
		return filename[0]
	}
	return dflt
}

// streamedMultipart builds a streamed multipart/form-data body by hand (avoids
// buffering a large stream into memory). The filename is embedded in a
// Content-Disposition header, so CR/LF, quotes and NULs are stripped to prevent
// header injection — the bytes themselves are never altered. If src is an
// io.Closer it is closed once fully read.
func streamedMultipart(boundary, filename string, src io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		// Close src on every exit path. CreatePart writes to the unbuffered pipe and
		// fails whenever the HTTP layer aborts before consuming the body (connection
		// refused, DNS failure); without this defer a file-path input would leak an
		// open file descriptor on each such early failure.
		defer closeIfCloser(src)
		mw := multipart.NewWriter(pw)
		if err := mw.SetBoundary(boundary); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		header := make(textproto.MIMEHeader)
		safe := sanitizeFilename(filename)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, safe))
		header.Set("Content-Type", "application/octet-stream")
		part, err := mw.CreatePart(header)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, src); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := mw.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr
}

func closeIfCloser(r io.Reader) {
	if c, ok := r.(io.Closer); ok {
		_ = c.Close()
	}
}

func sanitizeFilename(name string) string {
	return strings.NewReplacer("\r", "", "\n", "", `"`, "", "\x00", "").Replace(name)
}

func randomBoundary() string {
	var buf [16]byte
	_, _ = crand.Read(buf[:])
	return "A2CFormBoundary" + hex.EncodeToString(buf[:])
}

// bytesReadCloser adapts a byte slice to an io.ReadCloser so streamedMultipart's
// close-when-done path is uniform.
type bytesReadCloser struct {
	data []byte
	pos  int
}

func newBytesReadCloser(b []byte) *bytesReadCloser { return &bytesReadCloser{data: b} }

func (b *bytesReadCloser) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadCloser) Close() error { return nil }
