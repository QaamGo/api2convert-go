package api2convert_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// erroringBody delivers data, then fails with a raw (untyped) network-style error
// — simulating a connection reset partway through a download.
type erroringBody struct {
	data []byte
	pos  int
	err  error
}

func (b *erroringBody) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, b.err
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *erroringBody) Close() error { return nil }

// bodySender is a minimal HttpSender that returns a fixed 200 whose body is the
// supplied reader — lets a test drive the download read loop with a body that
// fails mid-stream.
type bodySender struct{ body io.ReadCloser }

func (s *bodySender) Send(_ context.Context, _ *api2convert.Request) (*api2convert.Response, error) {
	return &api2convert.Response{Status: 200, StatusText: "OK", Header: http.Header{}, Body: s.body}, nil
}

// TestSaveTypesMidStreamReadErrorAsNetwork proves a mid-download read failure is a
// typed *NetworkError (M4) and leaves no partial file behind (H3).
func TestSaveTypesMidStreamReadErrorAsNetwork(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")
	rawErr := errors.New("read tcp 10.0.0.1:443: connection reset by peer")
	sender := &bodySender{body: &erroringBody{data: []byte("partial payload"), err: rawErr}}
	c, err := api2convert.New("k", api2convert.WithHTTPSender(sender), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	out := api2convert.OutputFileOf("", "https://dl/x", "out.bin")
	_, gotErr := c.Download(out).Save(context.Background(), target)
	if gotErr == nil {
		t.Fatal("expected a mid-stream download error")
	}
	var ne *api2convert.NetworkError
	if !errors.As(gotErr, &ne) {
		t.Fatalf("err = %T (%v), want *NetworkError", gotErr, gotErr)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("a partial file was left at %q (stat err = %v)", target, statErr)
	}
}

// TestSaveFailurePreservesExistingFile proves a failed download never destroys a
// pre-existing complete file at the target path (H3 — no up-front truncation).
func TestSaveFailurePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")
	const original = "PREEXISTING COMPLETE FILE"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	sender := &bodySender{body: &erroringBody{data: []byte("partial"), err: errors.New("connection reset")}}
	c, err := api2convert.New("k", api2convert.WithHTTPSender(sender), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	out := api2convert.OutputFileOf("", "https://dl/x", "out.bin")
	if _, gotErr := c.Download(out).Save(context.Background(), target); gotErr == nil {
		t.Fatal("expected a mid-stream download error")
	}
	got, _ := os.ReadFile(target)
	if string(got) != original {
		t.Fatalf("a failed download clobbered the pre-existing file: got %q, want %q", got, original)
	}
}
