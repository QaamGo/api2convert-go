package security_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// TestSlowDownloadNotCappedByPerRequestTimeout proves the per-request timeout
// bounds only the pre-body phase (connect / TLS / response headers), never a
// healthy but slow body transfer (H2). The server flushes the response headers
// immediately, then delivers the body only after a delay longer than the
// configured timeout; the download must still complete. Uses the real net/http
// sender (no injected fake) so the actual timeout semantics are exercised.
func TestSlowDownloadNotCappedByPerRequestTimeout(t *testing.T) {
	const body = "the complete file, delivered only after a delay"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // headers arrive at once — within the header timeout
		time.Sleep(1500 * time.Millisecond)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c, err := api2convert.New("k",
		api2convert.WithTimeout(1*time.Second), // pre-body budget only (floored minimum)
		api2convert.WithMaxRetries(0),
	)
	if err != nil {
		t.Fatal(err)
	}

	out := api2convert.OutputFileOf("", srv.URL+"/file", "f")
	got, err := c.Download(out).Contents(context.Background())
	if err != nil {
		t.Fatalf("a slow but healthy body transfer was capped by the per-request timeout: %v", err)
	}
	if string(got) != body {
		t.Fatalf("contents = %q, want %q", got, body)
	}
}
