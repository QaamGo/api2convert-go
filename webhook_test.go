package api2convert_test

import (
	"errors"
	"strings"
	"testing"

	api2convert "github.com/QaamGo/api2convert-go/v10"
	"github.com/QaamGo/api2convert-go/v10/internal/testutil"
)

const whSecret = "whsec_test"

var whPayload = []byte(`{"id":"job-1","status":{"code":"completed"}}`)

func TestWebhooksUsableWithoutClient(t *testing.T) {
	sig := testutil.Sign(string(whPayload), whSecret)
	event, err := api2convert.Webhooks().ConstructEvent(whPayload, sig, whSecret)
	if err != nil {
		t.Fatal(err)
	}
	if event.Job.ID != "job-1" || !event.Job.IsCompleted() {
		t.Fatalf("event = %+v", event)
	}
}

func TestWebhookRejectsTamperedPayload(t *testing.T) {
	sig := testutil.Sign(string(whPayload), whSecret)
	_, err := api2convert.Webhooks().ConstructEvent(append(whPayload, ' '), sig, whSecret)
	var sve *api2convert.SignatureVerificationError
	if !errors.As(err, &sve) {
		t.Fatalf("err = %T, want *SignatureVerificationError", err)
	}
}

func TestWebhookRejectsMissingSignatureWhenSecretGiven(t *testing.T) {
	_, err := api2convert.Webhooks().ConstructEvent(whPayload, "", whSecret)
	var sve *api2convert.SignatureVerificationError
	if !errors.As(err, &sve) {
		t.Fatalf("err = %T, want *SignatureVerificationError", err)
	}
}

func TestWebhookEmptySecretBypassesVerification(t *testing.T) {
	event, err := api2convert.Webhooks().ConstructEvent(whPayload, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Job.ID != "job-1" {
		t.Fatalf("event = %+v", event)
	}
}

func TestWebhookRejectsEqualLengthWrongSignature(t *testing.T) {
	sig := testutil.Sign(string(whPayload), whSecret)
	wrong := strings.Repeat("f", len(sig)) // same length, wrong value: constant-time path, no crash
	_, err := api2convert.Webhooks().ConstructEvent(whPayload, wrong, whSecret)
	var sve *api2convert.SignatureVerificationError
	if !errors.As(err, &sve) {
		t.Fatalf("err = %T, want *SignatureVerificationError", err)
	}
}

func TestWebhookParseRejectsInvalidAndNonObjectJSON(t *testing.T) {
	for _, bad := range [][]byte{[]byte("{not json"), []byte("[1,2,3]"), []byte("42")} {
		if _, err := api2convert.Webhooks().Parse(bad); err == nil {
			t.Fatalf("Parse(%q) should error", bad)
		}
	}
}
