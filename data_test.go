package api2convert

import (
	"encoding/json"
	"reflect"
	"testing"
)

// White-box tests for the tolerant hydration helpers (support/data analog). These
// stay in-package because the helpers are unexported; they import no test helper,
// so there is no import cycle.

func TestAsStringOnlyAcceptsRealStrings(t *testing.T) {
	if got := asString("hello", "d"); got != "hello" {
		t.Fatalf("asString string = %q", got)
	}
	for _, v := range []any{nil, 3.0, true, json.Number("5"), []any{}} {
		if got := asString(v, "d"); got != "d" {
			t.Fatalf("asString(%v) = %q, want default", v, got)
		}
	}
}

func TestNullableInt64Coercion(t *testing.T) {
	cases := []struct {
		in   any
		want *int64
	}{
		{json.Number("12345"), ptrInt64(12345)},
		{json.Number("3.9"), ptrInt64(3)},
		{float64(42), ptrInt64(42)},
		{"678", ptrInt64(678)},
		{"  90 ", ptrInt64(90)},
		{"", nil},
		{"not-a-number", nil},
		{true, nil}, // booleans are rejected (is_numeric(true) == false)
		{nil, nil},
		{[]any{1}, nil},
	}
	for _, c := range cases {
		got := nullableInt64(c.in)
		if !eqInt64Ptr(got, c.want) {
			t.Fatalf("nullableInt64(%v=%T) = %v, want %v", c.in, c.in, deref(got), deref(c.want))
		}
	}
}

func TestNullableInt64PreservesLargePrecision(t *testing.T) {
	// A value past 2^53 must not lose precision. The SDK decodes JSON with
	// UseNumber(), so numbers arrive as json.Number and are parsed as exact ints.
	got := nullableInt64(json.Number("9007199254740993"))
	if got == nil || *got != 9007199254740993 {
		t.Fatalf("large size lost precision: %v", deref(got))
	}
}

func TestAsListArrayObjectAndOther(t *testing.T) {
	if got := asList([]any{1, 2}); len(got) != 2 {
		t.Fatalf("asList array len = %d", len(got))
	}
	// Object reduced to values ordered by key (deterministic).
	got := asList(map[string]any{"b": 2, "a": 1})
	if !reflect.DeepEqual(got, []any{1, 2}) {
		t.Fatalf("asList object = %v, want [1 2] (key-ordered)", got)
	}
	if got := asList("nope"); got != nil {
		t.Fatalf("asList(other) = %v, want nil", got)
	}
}

func TestMapObjectsSkipsNonObjects(t *testing.T) {
	in := []any{
		map[string]any{"code": "a"},
		"not-an-object",
		map[string]any{"code": "b"},
	}
	got := mapObjects(in, StatusFromMap)
	if len(got) != 2 || got[0].Code != "a" || got[1].Code != "b" {
		t.Fatalf("mapObjects = %+v", got)
	}
}

func ptrInt64(v int64) *int64 { return &v }
func eqInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
func deref(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
