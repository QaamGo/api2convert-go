package api2convert_test

import (
	"context"
	"strings"
	"testing"

	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

func TestJobsListUpdateCancelAddInputOutputs(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, []any{
			map[string]any{"id": "j1", "status": map[string]any{"code": "completed"}},
			map[string]any{"id": "j2", "status": map[string]any{"code": "processing"}},
		}).
		AddJSON(200, map[string]any{"id": "j1", "status": map[string]any{"code": "queued"}}).
		AddJSON(204, map[string]any{}).
		AddJSON(200, map[string]any{"id": "in-1", "type": "remote", "source": "https://x/y"}).
		AddJSON(200, []any{map[string]any{"id": "o1", "uri": "https://dl/o1"}})

	ctx := context.Background()

	jobs, err := tc.Client.Jobs().List(ctx, "completed", 2)
	if err != nil || len(jobs) != 2 || jobs[0].ID != "j1" {
		t.Fatalf("List = %+v err=%v", jobs, err)
	}
	if url := tc.HTTP.At(0).URL; url == "" || !contains(url, "status=completed") || !contains(url, "page=2") {
		t.Fatalf("List query = %q", url)
	}

	if _, err := tc.Client.Jobs().Update(ctx, "j1", map[string]any{"process": true}); err != nil {
		t.Fatal(err)
	}
	if r := tc.HTTP.At(1); r.Method != "PATCH" || r.URL[len(r.URL)-3:] != "/j1" {
		t.Fatalf("Update request = %+v", r)
	}

	if err := tc.Client.Jobs().Cancel(ctx, "j1"); err != nil {
		t.Fatal(err)
	}
	if r := tc.HTTP.At(2); r.Method != "DELETE" {
		t.Fatalf("Cancel method = %s", r.Method)
	}

	in, err := tc.Client.Jobs().AddInput(ctx, "j1", map[string]any{"type": "remote", "source": "https://x/y"})
	if err != nil || in.Type != "remote" || in.Source != "https://x/y" {
		t.Fatalf("AddInput = %+v err=%v", in, err)
	}

	outs, err := tc.Client.Jobs().Outputs(ctx, "j1")
	if err != nil || len(outs) != 1 || outs[0].URI != "https://dl/o1" {
		t.Fatalf("Outputs = %+v err=%v", outs, err)
	}
}

func TestConversionsListAndOptions(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, []any{map[string]any{"id": "c1", "category": "image", "target": "png", "options": map[string]any{"quality": map[string]any{"type": "integer"}}}}).
		AddJSON(200, []any{map[string]any{"target": "png", "options": map[string]any{"width": map[string]any{"type": "integer"}}}})

	ctx := context.Background()
	rows, err := tc.Client.Conversions().List(ctx, "image", "png", 1)
	if err != nil || len(rows) != 1 || rows[0]["target"] != "png" {
		t.Fatalf("List = %+v err=%v", rows, err)
	}
	if url := tc.HTTP.At(0).URL; !contains(url, "category=image") || !contains(url, "target=png") {
		t.Fatalf("conversions query = %q", url)
	}

	opts, err := tc.Client.Conversions().Options(ctx, "png")
	if err != nil || opts["width"] == nil {
		t.Fatalf("Options = %+v err=%v", opts, err)
	}
}

func TestPresetsCRUD(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, []any{map[string]any{"id": "p1", "name": "web", "target": "jpg"}}).
		AddJSON(201, map[string]any{"id": "p2", "name": "print", "target": "pdf"}).
		AddJSON(200, map[string]any{"id": "p2", "name": "print", "target": "pdf"}).
		AddJSON(200, map[string]any{"id": "p2", "name": "print2", "target": "pdf"}).
		AddJSON(204, map[string]any{})

	ctx := context.Background()
	list, err := tc.Client.Presets().List(ctx, "", "", "")
	if err != nil || len(list) != 1 || list[0].Name != "web" {
		t.Fatalf("List = %+v err=%v", list, err)
	}
	created, err := tc.Client.Presets().Create(ctx, map[string]any{"name": "print", "target": "pdf"})
	if err != nil || created.ID != "p2" {
		t.Fatalf("Create = %+v err=%v", created, err)
	}
	got, err := tc.Client.Presets().Get(ctx, "p2")
	if err != nil || got.Name != "print" {
		t.Fatalf("Get = %+v err=%v", got, err)
	}
	upd, err := tc.Client.Presets().Update(ctx, "p2", map[string]any{"name": "print2"})
	if err != nil || upd.Name != "print2" {
		t.Fatalf("Update = %+v err=%v", upd, err)
	}
	if err := tc.Client.Presets().Delete(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	if r := tc.HTTP.At(4); r.Method != "DELETE" {
		t.Fatalf("Delete method = %s", r.Method)
	}
}

func TestStatsAndContracts(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, map[string]any{"conversions": 10}).
		AddJSON(200, map[string]any{"conversions": 300}).
		AddJSON(200, map[string]any{"conversions": 3650}).
		AddJSON(200, map[string]any{"plan": "pro"})

	ctx := context.Background()
	if _, err := tc.Client.Stats().Day(ctx, "2026-07-04", ""); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(0).URL; !contains(url, "/stats/day/2026-07-04/all") {
		t.Fatalf("Day URL = %q (empty filter should default to 'all')", url)
	}
	if _, err := tc.Client.Stats().Month(ctx, "2026-07", "self"); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(1).URL; !contains(url, "/stats/month/2026-07/self") {
		t.Fatalf("Month URL = %q", url)
	}
	if _, err := tc.Client.Stats().Year(ctx, "2026", "all"); err != nil {
		t.Fatal(err)
	}
	if _, err := tc.Client.Contracts().Get(ctx); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(3).URL; !contains(url, "/contracts") {
		t.Fatalf("Contracts URL = %q", url)
	}
}

func TestPathSegmentsAreEncoded(t *testing.T) {
	tc := testutil.NewTestClient()
	tc.HTTP.
		AddJSON(200, map[string]any{"id": "j1"}).
		AddJSON(200, map[string]any{"id": "p1"}).
		AddJSON(200, map[string]any{"conversions": 1})

	ctx := context.Background()
	// A raw "/" "?" or "#" in a caller-supplied id must not alter the path or
	// inject a query/fragment — each segment is percent-encoded.
	const nasty = "a/b?c#d"
	const encoded = "a%2Fb%3Fc%23d"

	if _, err := tc.Client.Jobs().Get(ctx, nasty); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(0).URL; !contains(url, "/jobs/"+encoded) || contains(url, nasty) {
		t.Fatalf("Jobs.Get URL = %q, want encoded segment %q", url, encoded)
	}

	if _, err := tc.Client.Presets().Get(ctx, nasty); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(1).URL; !contains(url, "/presets/"+encoded) || contains(url, nasty) {
		t.Fatalf("Presets.Get URL = %q, want encoded segment %q", url, encoded)
	}

	if _, err := tc.Client.Stats().Day(ctx, nasty, nasty); err != nil {
		t.Fatal(err)
	}
	if url := tc.HTTP.At(2).URL; !contains(url, "/stats/day/"+encoded+"/"+encoded) || contains(url, nasty) {
		t.Fatalf("Stats.Day URL = %q, want encoded segments %q", url, encoded)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
