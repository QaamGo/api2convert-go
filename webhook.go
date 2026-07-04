package api2convert

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Webhook callback verification and parsing.
//
// Pass the raw request body (the exact bytes received) so signature verification
// is byte-exact. Verification uses HMAC-SHA256 and matches the server's
// signed-webhooks scheme; until signed webhooks are enabled on your account no
// signature is sent — use Parse then, or call ConstructEvent with an empty secret
// to skip verification. The signature is delivered in the X-Oc-Signature header.

// WebhookEvent is a verified webhook callback. The API posts the job whose status
// changed.
type WebhookEvent struct {
	// Job is the job whose status changed.
	Job Job
	// Payload is the full decoded callback body.
	Payload map[string]any
}

// WebhookVerifier verifies and parses webhook callbacks. Obtain one via
// api2convert.Webhooks(); it needs no configured client.
type WebhookVerifier struct{}

// ConstructEvent verifies the signature (when a secret is given) and returns the
// typed event.
//
// payload must be the raw request body. signature is the value of the signature
// header (X-Oc-Signature). Pass an empty secret to skip verification. Returns a
// *SignatureVerificationError when the signature is missing or does not match
// (constant-time comparison via hmac.Equal).
func (WebhookVerifier) ConstructEvent(payload []byte, signature, secret string) (*WebhookEvent, error) {
	if secret != "" {
		if signature == "" {
			return nil, &SignatureVerificationError{genericError{Message: "Missing webhook signature header."}}
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		// hmac.Equal is constant-time and safe for unequal-length inputs.
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return nil, &SignatureVerificationError{genericError{Message: "Webhook signature verification failed."}}
		}
	}
	return WebhookVerifier{}.Parse(payload)
}

// Parse parses a callback body into a typed event WITHOUT verifying a signature.
// Only use this when signed webhooks are not yet enabled for your account.
func (WebhookVerifier) Parse(payload []byte) (*WebhookEvent, error) {
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, &SignatureVerificationError{genericError{Message: "Webhook payload is not valid JSON: " + err.Error(), Cause: err}}
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, &SignatureVerificationError{genericError{Message: "Webhook payload is not a JSON object."}}
	}
	return &WebhookEvent{Job: JobFromMap(m), Payload: m}, nil
}
