package api2convert_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func stagedJob() api2convert.Job {
	return api2convert.JobFromMap(map[string]any{
		"id": "job-9", "token": "tok-abc", "server": "https://up.example.com/v2/",
		"status": map[string]any{"code": "incomplete"},
	})
}

func TestUploadPostsMultipartToJobServerWithToken(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(src, []byte("FILEDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "in-1", "type": "upload"})

	in, err := tc.Client.Jobs().Upload(context.Background(), stagedJob(), src)
	if err != nil {
		t.Fatal(err)
	}
	if in.ID != "in-1" {
		t.Fatalf("input id = %q", in.ID)
	}
	req := tc.HTTP.At(0)
	if req.Method != "POST" || req.URL != "https://up.example.com/v2/upload-file/job-9" {
		t.Fatalf("upload request = %+v", req)
	}
	if !strings.HasPrefix(req.H("Content-Type"), "multipart/form-data; boundary=") {
		t.Fatalf("content-type = %q", req.H("Content-Type"))
	}
	if !strings.Contains(string(req.Body), "FILEDATA") {
		t.Fatalf("body missing file bytes: %q", req.Body)
	}
	if !strings.Contains(string(req.Body), `filename="doc.txt"`) {
		t.Fatalf("body missing filename: %q", req.Body)
	}
}

func TestUploadNeverSendsAccountKeyAndDoesNotRedirect(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "in-1", "type": "upload"})

	if _, err := tc.Client.Jobs().Upload(context.Background(), stagedJob(), []byte("bytes")); err != nil {
		t.Fatal(err)
	}
	req := tc.HTTP.At(0)
	if req.H("X-Oc-Token") != "tok-abc" {
		t.Fatalf("upload must authenticate with the job token, got %q", req.H("X-Oc-Token"))
	}
	if req.H("X-Oc-Api-Key") != "" {
		t.Fatal("the account key must never reach the upload server")
	}
	if req.FollowRedirects {
		t.Fatal("an authenticated upload must not follow redirects")
	}
}

func TestUploadFailsWhenJobHasNoServerOrToken(t *testing.T) {
	tc := testutil.NewTestClient()
	job := api2convert.JobFromMap(map[string]any{"id": "job-1", "status": map[string]any{"code": "created"}})

	_, err := tc.Client.Jobs().Upload(context.Background(), job, []byte("x"))
	var a2c api2convert.Api2ConvertError
	if !errors.As(err, &a2c) {
		t.Fatalf("err = %T, want an SDK error", err)
	}
	if tc.HTTP.Count() != 0 {
		t.Fatalf("no request should be sent when the job has no upload server: %d", tc.HTTP.Count())
	}
}

func TestUploadRejectsFilenameHeaderInjection(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.AddJSON(200, map[string]any{"id": "in-1"})

	// A hostile filename with CR/LF must not inject extra multipart header lines.
	if _, err := tc.Client.Jobs().Upload(context.Background(), stagedJob(), []byte("x"), "e\r\nX-Evil: 1\r\n.png"); err != nil {
		t.Fatal(err)
	}
	body := string(tc.HTTP.At(0).Body)
	// Stripping CR/LF folds the value onto the single Content-Disposition line, so
	// no injected "\r\nX-Evil:" header can appear.
	if strings.Contains(body, "\r\nX-Evil") {
		t.Fatalf("CR/LF in filename must be stripped so no header line is injected, body = %q", body)
	}
	if !strings.Contains(body, `name="file"`) {
		t.Fatalf("multipart field header malformed: %q", body)
	}
}
