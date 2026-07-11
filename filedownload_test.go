package api2convert_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func TestSaveUsesAPIFilenameWhenTargetIsDir(t *testing.T) {
	dir := t.TempDir()
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("BYTES"))

	out := api2convert.OutputFileOf("", "https://dl/x", "result.png")
	path, err := tc.Client.Download(out).Save(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "result.png"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "BYTES" {
		t.Fatalf("contents = %q", data)
	}
	// A passwordless download follows redirects and carries no secret header.
	req := tc.HTTP.Last()
	if !req.FollowRedirects {
		t.Fatal("a no-auth download should follow redirects")
	}
	if req.H("X-Api2convert-Api-Key") != "" || req.H("X-Api2convert-Download-Password") != "" {
		t.Fatal("a passwordless download must carry no secret header")
	}
}

func TestSaveTraversalFilenameReducedToBasename(t *testing.T) {
	dir := t.TempDir()
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("X"))

	out := api2convert.OutputFileOf("", "https://dl/x", "../../../etc/evil")
	path, err := tc.Client.Download(out).Save(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "evil"); path != want {
		t.Fatalf("path = %q, want %q (must not escape the target dir)", path, want)
	}
}

func TestSaveFallsBackToOutputWhenNameUnusable(t *testing.T) {
	dir := t.TempDir()
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("X"))

	out := api2convert.OutputFileOf("", "https://dl/x", "") // no filename, no id
	path, err := tc.Client.Download(out).Save(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "output"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestSaveUsesExplicitFilePathVerbatim(t *testing.T) {
	dir := t.TempDir()
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("X"))

	target := filepath.Join(dir, "sub", "out.png") // parent must be created
	out := api2convert.OutputFileOf("", "https://dl/x", "ignored.png")
	path, err := tc.Client.Download(out).Save(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if path != target {
		t.Fatalf("path = %q, want %q", path, target)
	}
}

func TestSaveMakesNoRequestWhenDirCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	// A regular file where a directory is needed makes MkdirAll fail.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := testutil.NewTestClient(t)

	out := api2convert.OutputFileOf("", "https://dl/x", "out.png")
	_, err := tc.Client.Download(out).Save(context.Background(), filepath.Join(blocker, "out.png"))
	if err == nil {
		t.Fatal("expected an error when the target directory cannot be created")
	}
	if tc.HTTP.Count() != 0 {
		t.Fatalf("no download request should be made when mkdir fails: %d requests", tc.HTTP.Count())
	}
}

func TestContentsLoadsIntoMemory(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("HELLO"))

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	data, err := tc.Client.Download(out).Contents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "HELLO" {
		t.Fatalf("contents = %q", data)
	}
}

func TestDownloadWithPasswordSendsHeaderAndDoesNotFollowRedirects(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddRaw(200, []byte("SECRETBYTES"))

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	if _, err := tc.Client.Download(out, "s3cret").Contents(context.Background()); err != nil {
		t.Fatal(err)
	}
	req := tc.HTTP.Last()
	if req.FollowRedirects {
		t.Fatal("a password-protected download must not follow redirects")
	}
	if req.H("X-Api2convert-Download-Password") != "s3cret" {
		t.Fatalf("download password header = %q", req.H("X-Api2convert-Download-Password"))
	}
}

func TestSaveRemovesPartialFileOnCopyError(t *testing.T) {
	dir := t.TempDir()
	// A tiny download cap makes io.Copy fail partway through streaming the body.
	tc := testutil.NewTestClient(t, api2convert.WithMaxDownloadBytes(4))
	tc.HTTP.AddRaw(200, []byte("way too many bytes to fit under the cap"))

	target := filepath.Join(dir, "out.bin")
	out := api2convert.OutputFileOf("", "https://dl/x", "ignored")
	_, err := tc.Client.Download(out).Save(context.Background(), target)
	if err == nil {
		t.Fatal("expected an error when the copy fails")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("a failed download must leave no file at target; os.Stat err = %v", statErr)
	}
}

func TestMaxDownloadBytesRejectsOversizedBody(t *testing.T) {
	tc := testutil.NewTestClient(t, api2convert.WithMaxDownloadBytes(4))
	tc.HTTP.AddRaw(200, []byte("way too many bytes"))

	out := api2convert.OutputFileOf("", "https://dl/x", "f")
	_, err := tc.Client.Download(out).Contents(context.Background())
	var ne *api2convert.NetworkError
	if err == nil {
		t.Fatal("expected an error for an oversized download")
	}
	if !errors.As(err, &ne) {
		t.Fatalf("err = %T, want *NetworkError", err)
	}
}
