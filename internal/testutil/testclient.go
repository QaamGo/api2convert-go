package testutil

import (
	"context"
	"sync"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// Sleeper is a recording, instant sleeper: it never actually waits, but records
// every requested duration so backoff/poll tests can assert on them.
type Sleeper struct {
	mu        sync.Mutex
	durations []time.Duration
}

// Sleep records d and returns immediately.
func (s *Sleeper) Sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	s.durations = append(s.durations, d)
	s.mu.Unlock()
	return nil
}

// Durations returns a snapshot of the recorded sleep durations.
func (s *Sleeper) Durations() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.durations))
	copy(out, s.durations)
	return out
}

// TestClient bundles a client wired to a FakeSender and a recording Sleeper, with
// jitter disabled (rng == 0) for deterministic backoff assertions.
type TestClient struct {
	Client  *api2convert.Client
	HTTP    *FakeSender
	Sleeper *Sleeper
}

// NewTestClient builds a TestClient with the API key "test-key". Additional
// options are appended after the injected seams, so a caller may override, for
// example, maxRetries. It panics only on an impossible construction error (the
// non-empty key guarantees success).
func NewTestClient(opts ...api2convert.Option) *TestClient {
	http := &FakeSender{}
	sleeper := &Sleeper{}
	base := []api2convert.Option{
		api2convert.WithHTTPSender(http),
		api2convert.WithSleeper(sleeper.Sleep),
		api2convert.WithRand(func() float64 { return 0 }),
	}
	c, err := api2convert.New("test-key", append(base, opts...)...)
	if err != nil {
		panic(err)
	}
	return &TestClient{Client: c, HTTP: http, Sleeper: sleeper}
}
