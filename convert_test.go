package api2convert_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func TestConvertFromURLCreatesStartedJobPollsAndReturns(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "downloading"}}).
		AddJSON(200, map[string]any{
			"id":     "job-1",
			"status": map[string]any{"code": "completed"},
			"output": []any{map[string]any{"uri": "https://dl.example.com/out.png"}},
		})

	res, err := tc.Client.Convert(context.Background(), "https://example.com/in.jpg", "png")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Job.IsCompleted() {
		t.Fatalf("job not completed: %+v", res.Job.Status)
	}

	create := tc.HTTP.At(0)
	if create.Method != "POST" || create.H("X-Oc-Api-Key") != "test-key" {
		t.Fatalf("create request = %+v", create)
	}
	if create.FollowRedirects {
		t.Fatal("authenticated create must not follow redirects")
	}
	var body map[string]any
	if err := create.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["process"] != true {
		t.Fatalf("URL input must be a started job: process=%v", body["process"])
	}
	input, _ := body["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("expected one remote input, got %v", body["input"])
	}
	in0 := input[0].(map[string]any)
	if in0["type"] != "remote" || in0["source"] != "https://example.com/in.jpg" {
		t.Fatalf("remote input = %v", in0)
	}
	if poll := tc.HTTP.At(1); poll.Method != "GET" {
		t.Fatalf("expected poll GET, got %s", poll.Method)
	}
}

func TestConvertFromLocalPathUploadsThenStarts(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(src, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}

	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "token": "tok-abc", "server": "https://up.example.com/v2", "status": map[string]any{"code": "incomplete"}}).
		AddJSON(200, map[string]any{"id": "in-1", "type": "upload"}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "queued"}}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "completed"}, "output": []any{map[string]any{"uri": "https://dl/x"}}})

	if _, err := tc.Client.Convert(context.Background(), src, "jpg"); err != nil {
		t.Fatal(err)
	}

	create := tc.HTTP.At(0)
	var body map[string]any
	_ = create.JSON(&body)
	if body["process"] != false {
		t.Fatalf("local input must stage the job first: process=%v", body["process"])
	}

	upload := tc.HTTP.At(1)
	if upload.Method != "POST" || upload.URL != "https://up.example.com/v2/upload-file/job-1" {
		t.Fatalf("upload request = %+v", upload)
	}
	if upload.H("X-Oc-Token") != "tok-abc" {
		t.Fatalf("upload must use the job token, got %q", upload.H("X-Oc-Token"))
	}
	if upload.H("X-Oc-Api-Key") != "" {
		t.Fatal("the account key must never reach the upload server")
	}
	if upload.FollowRedirects {
		t.Fatal("an authenticated upload must not follow redirects")
	}
	if upload.Replayable != true {
		t.Fatal("a file-path upload should be replayable")
	}
	if !bytes.Contains(upload.Body, []byte("PNGDATA")) {
		t.Fatalf("upload body missing file bytes: %q", upload.Body)
	}

	if start := tc.HTTP.At(2); start.Method != "PATCH" {
		t.Fatalf("expected start PATCH, got %s", start.Method)
	}
}

func TestConvertFromReaderUsesNonReplayableUpload(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "token": "tok", "server": "https://up/v2", "status": map[string]any{"code": "incomplete"}}).
		AddJSON(200, map[string]any{"id": "in-1", "type": "upload"}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "queued"}}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "completed"}, "output": []any{map[string]any{"uri": "https://dl/x"}}})

	if _, err := tc.Client.Convert(context.Background(), bytes.NewReader([]byte("STREAM")), "png"); err != nil {
		t.Fatal(err)
	}
	upload := tc.HTTP.At(1)
	if upload.Replayable {
		t.Fatal("an io.Reader upload must be one-shot (not replayable)")
	}
	if !bytes.Contains(upload.Body, []byte("STREAM")) {
		t.Fatalf("upload body missing stream bytes: %q", upload.Body)
	}
}

func TestConvertAsyncReturnsWithoutPollingAndSetsCallback(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "downloading"}})

	job, err := tc.Client.ConvertAsync(context.Background(), "https://example.com/in.jpg", "png",
		api2convert.WithCallback("https://hook.example.com/cb"))
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-1" {
		t.Fatalf("job id = %q", job.ID)
	}
	if tc.HTTP.Count() != 1 {
		t.Fatalf("convertAsync must not poll: %d requests", tc.HTTP.Count())
	}
	var body map[string]any
	_ = tc.HTTP.At(0).JSON(&body)
	if body["callback"] != "https://hook.example.com/cb" || body["notify_status"] != true {
		t.Fatalf("callback/notify_status not set: %v", body)
	}
}

func TestConvertPassesConversionOptionsAndCategory(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "downloading"}}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "completed"}, "output": []any{map[string]any{"uri": "https://dl/x"}}})

	_, err := tc.Client.Convert(context.Background(), "https://example.com/in.jpg", "jpg",
		api2convert.WithConversionOptions(map[string]any{"quality": 85}),
		api2convert.WithCategory("image"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = tc.HTTP.At(0).JSON(&body)
	conv := body["conversion"].([]any)[0].(map[string]any)
	if conv["target"] != "jpg" || conv["category"] != "image" {
		t.Fatalf("conversion = %v", conv)
	}
	opts, ok := conv["options"].(map[string]any)
	if !ok || opts["quality"].(float64) != 85 {
		t.Fatalf("options = %v", conv["options"])
	}
}
