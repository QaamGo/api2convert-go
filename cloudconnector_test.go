package api2convert_test

import (
	"context"
	"reflect"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// Cloud-connector parity fixtures 1 (create-payload serialization) and 2 (read
// hydration), plus the unit behaviour of the new cloud types. The JSON shapes and
// assertions mirror the canonical fixtures shared across every SDK
// (experiments/api2convert-cloud-connector-parity-fixtures.md).

// ---- Fixture 1: create-payload (what convert() serializes) ------------------------------

func TestFixture1ConvertSerializesCloudInputAndOutputTarget(t *testing.T) {
	tc := testutil.NewTestClient(t)
	// create → started job; Wait polls once to a completed job with no local output.
	tc.HTTP.
		AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "incomplete"}}).
		AddJSON(200, map[string]any{"id": "job-1", "status": map[string]any{"code": "completed"}})

	input := api2convert.CloudInputAmazonS3("my-bucket", "in/photo.png", "AKIA_TEST", "SECRET_TEST")
	target := api2convert.OutputTargetOf(
		api2convert.CloudProviderFtp,
		map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"},
		map[string]any{"username": "u", "password": "p"},
	)

	if _, err := tc.Client.Convert(context.Background(), input, "jpg",
		api2convert.WithOutputTargets(target)); err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := tc.HTTP.At(0).JSON(&body); err != nil {
		t.Fatal(err)
	}

	// 1) a cloud input is a started job (like a remote URL), not staged/uploaded.
	if body["process"] != true {
		t.Fatalf("cloud input must be a started job: process=%v", body["process"])
	}

	// 2) input[0] carries the flat/lowercase keys exactly as the factory emits them.
	wantInput := map[string]any{
		"type":        "cloud",
		"source":      "amazons3",
		"parameters":  map[string]any{"bucket": "my-bucket", "file": "in/photo.png"},
		"credentials": map[string]any{"accesskeyid": "AKIA_TEST", "secretaccesskey": "SECRET_TEST"},
	}
	gotInput := body["input"].([]any)
	if len(gotInput) != 1 || !reflect.DeepEqual(gotInput[0], wantInput) {
		t.Fatalf("input[0] = %#v, want %#v", body["input"], wantInput)
	}

	conv := body["conversion"].([]any)[0].(map[string]any)

	// 3) output_target[0] serializes {type,parameters,credentials} and NO status.
	wantTarget := map[string]any{
		"type":        "ftp",
		"parameters":  map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"},
		"credentials": map[string]any{"username": "u", "password": "p"},
	}
	gotTargets := conv["output_target"].([]any)
	if len(gotTargets) != 1 || !reflect.DeepEqual(gotTargets[0], wantTarget) {
		t.Fatalf("output_target[0] = %#v, want %#v", conv["output_target"], wantTarget)
	}
	if _, hasStatus := gotTargets[0].(map[string]any)["status"]; hasStatus {
		t.Fatal("output_target must not serialize a status key")
	}

	// output targets never leak into the conversion options map.
	if _, hasOptions := conv["options"]; hasOptions {
		t.Fatal("output targets must not surface as conversion options")
	}
}

func TestFixture1RawCreatePathProducesByteIdenticalOutputTarget(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddJSON(201, map[string]any{"id": "job-1", "status": map[string]any{"code": "completed"}})

	in := api2convert.CloudInputAmazonS3("my-bucket", "in/photo.png", "AKIA_TEST", "SECRET_TEST")
	target := api2convert.OutputTargetOf(
		api2convert.CloudProviderFtp,
		map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"},
		map[string]any{"username": "u", "password": "p"},
	)
	_, err := tc.Client.Jobs().Create(context.Background(), map[string]any{
		"process": true,
		"input":   []any{in.Descriptor()},
		"conversion": []any{map[string]any{
			"target":        "jpg",
			"output_target": []any{target.Descriptor()},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := tc.HTTP.At(0).JSON(&body); err != nil {
		t.Fatal(err)
	}
	wantInput := map[string]any{
		"type":        "cloud",
		"source":      "amazons3",
		"parameters":  map[string]any{"bucket": "my-bucket", "file": "in/photo.png"},
		"credentials": map[string]any{"accesskeyid": "AKIA_TEST", "secretaccesskey": "SECRET_TEST"},
	}
	wantTarget := map[string]any{
		"type":        "ftp",
		"parameters":  map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"},
		"credentials": map[string]any{"username": "u", "password": "p"},
	}
	// Both the convert() outputTargets control and the raw create map yield the same bytes.
	if got := body["input"].([]any); !reflect.DeepEqual(got[0], wantInput) {
		t.Fatalf("raw-path input[0] = %#v, want %#v", got[0], wantInput)
	}
	conv := body["conversion"].([]any)[0].(map[string]any)
	if got := conv["output_target"].([]any); !reflect.DeepEqual(got[0], wantTarget) {
		t.Fatalf("raw-path output_target[0] = %#v, want %#v", got[0], wantTarget)
	}
}

func TestAddInputAcceptsCloudInputDescriptor(t *testing.T) {
	tc := testutil.NewTestClient(t)
	tc.HTTP.AddJSON(200, map[string]any{"id": "in-1", "type": "cloud", "source": "ftp"})

	in := api2convert.CloudInputFTP("ftp.example.com", "in/a.png", "u", "p")
	if _, err := tc.Client.Jobs().AddInput(context.Background(), "job-1", in.Descriptor()); err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := tc.HTTP.At(0).JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["type"] != "cloud" || body["source"] != "ftp" {
		t.Fatalf("descriptor type/source = %v/%v", body["type"], body["source"])
	}
	if !reflect.DeepEqual(body["parameters"], map[string]any{"host": "ftp.example.com", "file": "in/a.png"}) {
		t.Fatalf("parameters = %#v", body["parameters"])
	}
	if !reflect.DeepEqual(body["credentials"], map[string]any{"username": "u", "password": "p"}) {
		t.Fatalf("credentials = %#v", body["credentials"])
	}
}

// ---- Fixture 2: read hydration (a GET /jobs/{id} response) ------------------------------

func TestFixture2HydratesCloudInputAndOutputTarget(t *testing.T) {
	job := api2convert.JobFromMap(map[string]any{
		"id":     "job-1",
		"status": map[string]any{"code": "completed"},
		"input": []any{map[string]any{
			"id":          "in-1",
			"type":        "cloud",
			"source":      "amazons3",
			"status":      "ready",
			"parameters":  map[string]any{"bucket": "my-bucket", "file": "in/photo.png"},
			"credentials": map[string]any{},
		}},
		"conversion": []any{map[string]any{
			"id":     "c-1",
			"target": "jpg",
			"output_target": []any{map[string]any{
				"type":        "ftp",
				"parameters":  map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"},
				"credentials": map[string]any{},
				"status":      "uploading",
			}},
		}},
	})

	// 1) input source is a RAW string; parameters surface.
	in := job.Input[0]
	if in.Source != "amazons3" || in.Status != "ready" {
		t.Fatalf("input source/status = %q/%q", in.Source, in.Status)
	}
	if !reflect.DeepEqual(in.Parameters, map[string]any{"bucket": "my-bucket", "file": "in/photo.png"}) {
		t.Fatalf("input parameters = %#v", in.Parameters)
	}

	// 2) output target type/status/parameters surface.
	if len(job.Conversion) != 1 || len(job.Conversion[0].OutputTargets) != 1 {
		t.Fatalf("expected one output target, got %#v", job.Conversion)
	}
	out := job.Conversion[0].OutputTargets[0]
	if out.Type != "ftp" || out.Status != "uploading" {
		t.Fatalf("output target type/status = %q/%q", out.Type, out.Status)
	}
	if !reflect.DeepEqual(out.Parameters, map[string]any{"host": "ftp.example.com", "file": "/out/photo.jpg"}) {
		t.Fatalf("output target parameters = %#v", out.Parameters)
	}

	// 3) credentials are never surfaced (the API returns them empty; the SDK does not hydrate).
	if len(out.Credentials) != 0 {
		t.Fatalf("credentials must not be surfaced, got %#v", out.Credentials)
	}
}

func TestFixture2UnknownProviderRoundTripsUntyped(t *testing.T) {
	job := api2convert.JobFromMap(map[string]any{
		"id":     "job-1",
		"status": map[string]any{"code": "completed"},
		"input":  []any{map[string]any{"id": "in-1", "type": "cloud", "source": "r2", "status": "ready"}},
		"conversion": []any{map[string]any{
			"target":        "jpg",
			"output_target": []any{map[string]any{"type": "r2", "status": "waiting"}},
		}},
	})

	// An unknown provider string hydrates without any parse throwing; the raw string round-trips.
	if job.Input[0].Source != "r2" {
		t.Fatalf("input source = %q, want r2", job.Input[0].Source)
	}
	out := job.Conversion[0].OutputTargets[0]
	if out.Type != "r2" || out.Status != "waiting" {
		t.Fatalf("output target type/status = %q/%q, want r2/waiting", out.Type, out.Status)
	}
}

// ---- Unit: the new value types ---------------------------------------------------------

func TestPerProviderConstructorsCarryRequiredKeysVerbatim(t *testing.T) {
	if got := api2convert.CloudInputAzure("c", "f", "n", "k").Descriptor(); !reflect.DeepEqual(got, map[string]any{
		"type":        "cloud",
		"source":      "azure",
		"parameters":  map[string]any{"container": "c", "file": "f"},
		"credentials": map[string]any{"accountname": "n", "accountkey": "k"},
	}) {
		t.Fatalf("azure descriptor = %#v", got)
	}
	if got := api2convert.CloudInputGoogleCloud("p", "b", "f", "kf").Descriptor(); !reflect.DeepEqual(got, map[string]any{
		"type":        "cloud",
		"source":      "googlecloud",
		"parameters":  map[string]any{"projectid": "p", "bucket": "b", "file": "f"},
		"credentials": map[string]any{"keyfile": "kf"},
	}) {
		t.Fatalf("googlecloud descriptor = %#v", got)
	}
}

func TestGenericEscapeHatchCarriesForwardCompatKeys(t *testing.T) {
	in := api2convert.CloudInputOf(
		api2convert.CloudProviderAmazonS3,
		map[string]any{"bucket": "b", "file": "f", "region": "eu"},
		map[string]any{"accesskeyid": "id", "secretaccesskey": "sec", "sessiontoken": "t"},
	)
	d := in.Descriptor()
	if !reflect.DeepEqual(d["parameters"], map[string]any{"bucket": "b", "file": "f", "region": "eu"}) {
		t.Fatalf("parameters = %#v", d["parameters"])
	}
	if !reflect.DeepEqual(d["credentials"], map[string]any{"accesskeyid": "id", "secretaccesskey": "sec", "sessiontoken": "t"}) {
		t.Fatalf("credentials = %#v", d["credentials"])
	}
	// A forward-compat provider string round-trips through the generic hatch too.
	if got := api2convert.CloudInputOf(api2convert.CloudProvider("r2"), nil, nil).Source; got != "r2" {
		t.Fatalf("forward-compat source = %q", got)
	}
}

func TestOutputTargetOmitsStatusOnSerializeButHydratesItOnRead(t *testing.T) {
	created := api2convert.OutputTarget{
		Type:        "ftp",
		Parameters:  map[string]any{"host": "h"},
		Credentials: map[string]any{"username": "u"},
		Status:      "completed",
	}
	if _, has := created.Descriptor()["status"]; has {
		t.Fatal("Descriptor must omit status")
	}

	read := api2convert.OutputTargetFromMap(map[string]any{
		"type":       "ftp",
		"parameters": map[string]any{"host": "h"},
		"status":     "completed",
	})
	if read.Status != "completed" {
		t.Fatalf("hydrated status = %q", read.Status)
	}
	if len(read.Credentials) != 0 {
		t.Fatalf("hydrated credentials must be empty, got %#v", read.Credentials)
	}
}
