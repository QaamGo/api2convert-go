package security_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

// Cloud-connector fixture 3 — the credential redaction / isolation suite.
//
// The single secret SUPERSECRET123 must never appear on any rendering/error path,
// and the fixed marker [REDACTED] must appear where a credentials object is
// rendered.

const (
	cloudSecret = "SUPERSECRET123"
	cloudMarker = "[REDACTED]"
)

// ---- 3a: object rendering --------------------------------------------------------------

func TestCloudInputRenderingMasksCredentials(t *testing.T) {
	rendered := fmt.Sprintf("%v", api2convert.CloudInputAmazonS3("b", "f", "AKIA", cloudSecret))

	if strings.Contains(rendered, cloudSecret) {
		t.Fatalf("secret leaked into CloudInput rendering: %q", rendered)
	}
	if !strings.Contains(rendered, cloudMarker) {
		t.Fatalf("expected the marker in CloudInput rendering: %q", rendered)
	}
	// Non-secret parameters still render.
	if !strings.Contains(rendered, `"bucket":"b"`) {
		t.Fatalf("non-secret parameter must render: %q", rendered)
	}
	// Also cover the plus-flag verbose verb and the Stringer path directly.
	if strings.Contains(fmt.Sprintf("%+v", api2convert.CloudInputAmazonS3("b", "f", "AKIA", cloudSecret)), cloudSecret) {
		t.Fatal("secret leaked into the verbose rendering of CloudInput")
	}
}

func TestOutputTargetRenderingMasksCredentials(t *testing.T) {
	rendered := api2convert.OutputTargetOf(
		api2convert.CloudProviderFtp,
		map[string]any{"host": "ftp.example.com"},
		map[string]any{"username": "u", "password": cloudSecret},
	).String()

	if strings.Contains(rendered, cloudSecret) {
		t.Fatalf("secret leaked into OutputTarget rendering: %q", rendered)
	}
	if !strings.Contains(rendered, cloudMarker) {
		t.Fatalf("expected the marker in OutputTarget rendering: %q", rendered)
	}
}

// ---- 3b + 3c: error text and error-body deep-walk --------------------------------------

func TestCreatePathErrorNeverLeaksSubmittedCredential(t *testing.T) {
	// A 422 whose decoded body echoes the submitted secret in a nested/dotted key
	// (belt-and-suspenders: the real API echoes field names only). The convert()
	// request body itself carried the secret in credentials — it must not surface on
	// the exception either.
	fake := &testutil.FakeSender{}
	c, err := api2convert.New("test-key", api2convert.WithHTTPSender(fake), api2convert.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	fake.AddJSON(422, map[string]any{
		"message": "Validation failed",
		"errors":  map[string]any{"input.0.credentials.secretaccesskey": cloudSecret},
	})

	_, gotErr := c.Convert(ctx(), api2convert.CloudInputAmazonS3("b", "f", "AKIA", cloudSecret), "jpg")

	// 3b: no secret in the message or anywhere on the exception rendering.
	assertNoSecret(t, gotErr, cloudSecret)

	// 3c: the deep-walk masks the echoed secret to the marker on the typed body.
	var he api2convert.HTTPError
	if !errors.As(gotErr, &he) {
		t.Fatalf("expected an HTTPError, got %T", gotErr)
	}
	encoded, _ := json.Marshal(he.Body())
	if strings.Contains(string(encoded), cloudSecret) {
		t.Fatalf("secret leaked into the error body: %s", encoded)
	}
	if !strings.Contains(string(encoded), cloudMarker) {
		t.Fatalf("expected the marker in the redacted error body: %s", encoded)
	}
}

// ---- 3d: sensitive parameters leaf -----------------------------------------------------

func TestSensitiveParametersLeafIsMaskedInRendering(t *testing.T) {
	rendered := fmt.Sprintf("%v", api2convert.CloudInputOf(
		api2convert.CloudProviderAmazonS3,
		map[string]any{"token": "PARAMSECRET", "bucket": "b"},
		nil,
	))

	if strings.Contains(rendered, "PARAMSECRET") {
		t.Fatalf("sensitive parameter leaked: %q", rendered)
	}
	if !strings.Contains(rendered, cloudMarker) {
		t.Fatalf("expected the marker: %q", rendered)
	}
	// A non-secret key renders normally.
	if !strings.Contains(rendered, `"bucket":"b"`) {
		t.Fatalf("non-secret key must render: %q", rendered)
	}
}
