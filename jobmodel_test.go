package api2convert_test

import (
	"encoding/json"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go"
)

func TestJobFromMapComputesTerminalPredicates(t *testing.T) {
	cases := []struct {
		code                                  string
		completed, failed, canceled, terminal bool
	}{
		{"completed", true, false, false, true},
		{"failed", false, true, false, true},
		{"canceled", false, false, true, true},
		{"processing", false, false, false, false},
		{"downloading", false, false, false, false},
		{"something-new", false, false, false, false}, // unknown code is non-terminal
	}
	for _, c := range cases {
		job := api2convert.JobFromMap(map[string]any{"status": map[string]any{"code": c.code}})
		if job.IsCompleted() != c.completed || job.IsFailed() != c.failed ||
			job.IsCanceled() != c.canceled || job.IsTerminal() != c.terminal {
			t.Fatalf("code %q predicates wrong: %+v", c.code, job)
		}
	}
}

func TestJobKeepsRawPayload(t *testing.T) {
	var m map[string]any
	_ = json.Unmarshal([]byte(`{"id":"j","status":{"code":"completed"},"future_field":123}`), &m)
	job := api2convert.JobFromMap(m)
	if job.Raw["future_field"] == nil {
		t.Fatal("Raw should preserve unknown fields for forward-compat")
	}
}

func TestOutputFileHydration(t *testing.T) {
	out := api2convert.OutputFileFromMap(map[string]any{
		"id":           "o1",
		"uri":          "https://dl/x",
		"filename":     "out.pdf",
		"size":         json.Number("12345"),
		"content_type": "application/pdf",
		"checksum":     "abc",
	})
	if out.URI != "https://dl/x" || out.Filename != "out.pdf" || out.ContentType != "application/pdf" {
		t.Fatalf("output = %+v", out)
	}
	if out.Size == nil || *out.Size != 12345 {
		t.Fatalf("size = %v", out.Size)
	}
}

func TestOutputFileOf(t *testing.T) {
	out := api2convert.OutputFileOf("id-1", "https://dl/x", "f.png")
	if out.ID != "id-1" || out.URI != "https://dl/x" || out.Filename != "f.png" {
		t.Fatalf("OutputFileOf = %+v", out)
	}
}
