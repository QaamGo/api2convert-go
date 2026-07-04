package api2convert

import "math/rand/v2"

// randFloat64 returns a [0,1) float. math/rand/v2's top-level source is
// concurrency-safe and needs no seeding.
func randFloat64() float64 { return rand.Float64() }
