// Package testutil provides shared test helpers for the api2convert SDK: an
// injectable fake HTTP sender, a recording sleeper, and real loopback servers for
// the security suite. It drives the SDK through its public options only
// (WithHTTPSender / WithSleeper / WithRand), so it never needs package-internal
// access.
package testutil

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	api2convert "github.com/QaamGo/api2convert-go"
)

// RecordedRequest is a captured outbound request (the Go analog of Node's
// RecordedRequest / Java's requestAt(i)).
type RecordedRequest struct {
	Method          string
	URL             string
	Header          http.Header
	FollowRedirects bool
	Replayable      bool
	Body            []byte
}

// H returns a request header by name (case-insensitive).
func (r RecordedRequest) H(name string) string { return r.Header.Get(name) }

// JSON unmarshals the recorded request body into v.
func (r RecordedRequest) JSON(v any) error { return json.Unmarshal(r.Body, v) }

type queued struct {
	status int
	body   []byte
	header http.Header
	err    error
}

// FakeSender implements api2convert.HttpSender: it records every request and
// returns canned responses from a FIFO queue. Real network I/O adds nothing for
// header/body/error assertions, so unit tests use this; the security suite uses
// real loopback servers only where a genuine redirect round-trip matters.
type FakeSender struct {
	mu       sync.Mutex
	requests []RecordedRequest
	queue    []queued
}

// Send records the request and returns the next queued response.
func (f *FakeSender) Send(_ context.Context, req *api2convert.Request) (*api2convert.Response, error) {
	var body []byte
	if req.MakeBody != nil {
		if r, err := req.MakeBody(); err == nil {
			body, _ = io.ReadAll(r)
		}
	} else if req.Body != nil {
		body = append([]byte(nil), req.Body...)
	}

	f.mu.Lock()
	f.requests = append(f.requests, RecordedRequest{
		Method:          req.Method,
		URL:             req.URL,
		Header:          headerFromMap(req.Headers),
		FollowRedirects: req.FollowRedirects,
		Replayable:      req.Replayable,
		Body:            body,
	})
	if len(f.queue) == 0 {
		f.mu.Unlock()
		return nil, fmt.Errorf("fakesender: no queued response for %s %s", req.Method, req.URL)
	}
	q := f.queue[0]
	f.queue = f.queue[1:]
	f.mu.Unlock()

	if q.err != nil {
		return nil, q.err
	}
	hdr := q.header
	if hdr == nil {
		hdr = http.Header{}
	}
	return &api2convert.Response{
		Status:     q.status,
		StatusText: http.StatusText(q.status),
		Header:     hdr,
		Body:       io.NopCloser(bytes.NewReader(q.body)),
	}, nil
}

// AddJSON queues a JSON response.
func (f *FakeSender) AddJSON(status int, body any, header ...http.Header) *FakeSender {
	b, _ := json.Marshal(body)
	return f.enqueue(queued{status: status, body: b, header: firstHeader(header)})
}

// AddRaw queues a raw-bytes response.
func (f *FakeSender) AddRaw(status int, b []byte, header ...http.Header) *FakeSender {
	return f.enqueue(queued{status: status, body: b, header: firstHeader(header)})
}

// AddText queues a text response.
func (f *FakeSender) AddText(status int, s string, header ...http.Header) *FakeSender {
	return f.enqueue(queued{status: status, body: []byte(s), header: firstHeader(header)})
}

// AddError queues a transport-level error (as if the network call failed).
func (f *FakeSender) AddError(err error) *FakeSender { return f.enqueue(queued{err: err}) }

func (f *FakeSender) enqueue(q queued) *FakeSender {
	f.mu.Lock()
	f.queue = append(f.queue, q)
	f.mu.Unlock()
	return f
}

// Requests returns a snapshot of all recorded requests.
func (f *FakeSender) Requests() []RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]RecordedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// Count returns the number of recorded requests.
func (f *FakeSender) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// At returns the i-th recorded request.
func (f *FakeSender) At(i int) RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[i]
}

// Last returns the most recently recorded request.
func (f *FakeSender) Last() RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func headerFromMap(m map[string]string) http.Header {
	h := http.Header{}
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}

func firstHeader(h []http.Header) http.Header {
	if len(h) > 0 {
		return h[0]
	}
	return nil
}

// Sign computes the hex HMAC-SHA256 the server uses to sign webhooks.
func Sign(payload, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}
