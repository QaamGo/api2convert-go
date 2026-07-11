package api2convert

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Typed, null-safe accessors over decoded JSON.
//
// Mirrors the Python SDK's _data / PHP Support\Data / Node support/data helpers:
// model hydration stays free of scattered type assertions and, crucially, never
// panics on a surprising payload — a missing or wrong-typed field falls back to a
// sensible default. Internal helpers, not part of the public API.
//
// JSON is decoded with json.Decoder.UseNumber(), so numeric values arrive as
// json.Number (preserving full int64 precision for large file sizes) rather than
// float64; the coercion helpers below accept json.Number, float64 and strings.

// asObject returns value when it is a JSON object, else an empty map.
func asObject(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// asString returns value when it is a real string, else dflt. It never
// stringifies numbers or booleans.
func asString(value any, dflt string) string {
	if s, ok := value.(string); ok {
		return s
	}
	return dflt
}

// asList returns a slice of values. A JSON array passes through; a JSON object is
// reduced to its values ordered by key (deterministic, mirrors PHP array_values);
// anything else yields nil.
func asList(value any) []any {
	switch t := value.(type) {
	case []any:
		return t
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(t))
		for _, k := range keys {
			out = append(out, t[k])
		}
		return out
	default:
		return nil
	}
}

// nullableInt64 coerces numeric values to a whole number (truncating toward zero),
// else nil. Booleans are rejected (mirrors PHP is_numeric(true) == false). Numeric
// strings and floats are truncated ("3.9" -> 3).
func nullableInt64(value any) *int64 {
	switch t := value.(type) {
	case bool:
		return nil
	case json.Number:
		// Prefer an exact integer parse (preserves precision past 2^53); fall back
		// to float truncation for fractional values ("3.9" -> 3).
		if n, err := t.Int64(); err == nil {
			return &n
		}
		return parseNumericString(t.String())
	case float64:
		return truncToInt64(t)
	case int:
		n := int64(t)
		return &n
	case int64:
		return &t
	case string:
		return parseNumericString(t)
	default:
		return nil
	}
}

func parseNumericString(s string) *int64 {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	// Exact integer parse first (no precision loss), then float truncation.
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return &n
	}
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil
	}
	return truncToInt64(f)
}

// truncToInt64 truncates f toward zero to an *int64, or returns nil when f is
// NaN/Inf or falls outside the int64 range. A bare int64(f) for an out-of-range
// float is implementation-defined (e.g. MinInt64 on amd64), which would hydrate
// garbage instead of signaling absence — defeating the tolerant-hydration promise.
func truncToInt64(f float64) *int64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	t := math.Trunc(f)
	// float64 cannot represent 2^63-1 exactly, so use 2^63 as the exclusive upper
	// bound; -2^63 (int64 min) is exactly representable and allowed.
	if t < -9223372036854775808.0 || t >= 9223372036854775808.0 {
		return nil
	}
	n := int64(t)
	return &n
}

// mapObjects builds a model from each object element of value; non-object elements
// are skipped.
func mapObjects[T any](value any, factory func(map[string]any) T) []T {
	list := asList(value)
	out := make([]T, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, factory(m))
		}
	}
	return out
}
