package api2convert_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// convertToResult drives a URL conversion to a completed ConversionResult, queuing
// the create + poll responses and leaving `downloadResponses` queued for the
// subsequent download(s).
func completedURLConvert(t *testing.T, opts ...api2convert.ConvertOption) (*api2convert.ConversionResult, *testutil.TestClient) {
	t.Helper()
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "downloading"}}).
		AddJSON(200, map[string]any{
			"id":     "job-1",
			"status": map[string]any{"code": "completed"},
			"output": []any{
				map[string]any{"id": "o1", "uri": "https://dl/o1", "filename": "a.png"},
				map[string]any{"id": "o2", "uri": "https://dl/o2", "filename": "b.png"},
			},
		})
	res, err := tc.Client.Convert(context.Background(), "https://example.com/in.jpg", "png", opts...)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	return res, tc
}

func TestConversionResultOutputsAndURL(t *testing.T) {
	res, _ := completedURLConvert(t)
	if outs := res.Outputs(); len(outs) != 2 {
		t.Fatalf("Outputs len = %d, want 2", len(outs))
	}
	out, err := res.Output()
	if err != nil || out.ID != "o1" {
		t.Fatalf("Output = %+v err=%v (default index 0)", out, err)
	}
	url, err := res.URL()
	if err != nil || url != "https://dl/o1" {
		t.Fatalf("URL = %q err=%v", url, err)
	}
}

func TestConversionResultSaveRemembersDownloadPassword(t *testing.T) {
	res, tc := completedURLConvert(t, api2convert.WithDownloadPassword("pw"))

	// The create payload must carry download_passwords.
	var createBody map[string]any
	_ = tc.HTTP.At(0).JSON(&createBody)
	if dp, ok := createBody["download_passwords"].([]any); !ok || len(dp) != 1 || dp[0] != "pw" {
		t.Fatalf("download_passwords not set on create: %v", createBody["download_passwords"])
	}

	tc.HTTP.AddRaw(200, []byte("BYTES"))
	dir := t.TempDir()
	path, err := res.Save(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "a.png"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	// The password supplied at conversion time is applied automatically on download.
	if got := tc.HTTP.Last().H("X-Oc-Download-Password"); got != "pw" {
		t.Fatalf("remembered download password not sent: %q", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "BYTES" {
		t.Fatalf("contents = %q", data)
	}
}

func TestConversionResultOutputIndexSelectsAndValidates(t *testing.T) {
	// A valid non-zero index selects the second output.
	res, _ := completedURLConvert(t, api2convert.WithOutputIndex(1))
	out, err := res.Output()
	if err != nil || out.ID != "o2" {
		t.Fatalf("Output(index 1) = %+v err=%v", out, err)
	}

	// An out-of-range index is an error, not a wrap-around.
	res2, _ := completedURLConvert(t, api2convert.WithOutputIndex(9))
	if _, err := res2.Output(); err == nil {
		t.Fatal("out-of-range output index should error")
	}
}

func TestConversionResultDownloadSpecificOutput(t *testing.T) {
	res, tc := completedURLConvert(t)
	tc.HTTP.AddRaw(200, []byte("SECOND"))

	dl, err := res.Download(res.Outputs()[1])
	if err != nil {
		t.Fatal(err)
	}
	data, err := dl.Contents(context.Background())
	if err != nil || string(data) != "SECOND" {
		t.Fatalf("contents = %q err=%v", data, err)
	}
	if tc.HTTP.Last().URL != "https://dl/o2" {
		t.Fatalf("downloaded wrong output: %q", tc.HTTP.Last().URL)
	}
}
