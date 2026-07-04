package testutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
)

// Recorder wraps an httptest.Server, counts hits and records each request's
// headers in order — the Go analog of Node's LoopbackServer and Java's
// AtomicInteger evilHits. It binds 127.0.0.1:0 so cross-host redirect guarantees
// can be proven against real servers (a mocked sender would short-circuit the
// redirect and prove nothing).
type Recorder struct {
	*httptest.Server
	hits    atomic.Int64
	mu      sync.Mutex
	headers []http.Header
}

// Hits returns the number of requests the server received.
func (r *Recorder) Hits() int { return int(r.hits.Load()) }

// Headers returns a snapshot of the recorded request headers, in order.
func (r *Recorder) Headers() []http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]http.Header, len(r.headers))
	copy(out, r.headers)
	return out
}

// StartServer starts a recording loopback server delegating to h. The caller must
// arrange for Close (e.g. t.Cleanup(rec.Close)).
func StartServer(h http.HandlerFunc) *Recorder {
	rec := &Recorder{}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec.hits.Add(1)
		rec.mu.Lock()
		rec.headers = append(rec.headers, req.Header.Clone())
		rec.mu.Unlock()
		h(w, req)
	}))
	return rec
}

// RedirectTo returns a handler that responds with a 302 to location.
func RedirectTo(location string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}
}

// Respond returns a handler that writes status and body.
func Respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}
