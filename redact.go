package api2convert

import (
	"encoding/json"
	"strings"
)

// Credential redaction for cloud connectors (D9).
//
// Cloud credentials ride in the plaintext request body, so they must never
// surface where a value object or an SDK-emitted string could leak them. This
// file centralizes the masks the contract mandates:
//
//   - the whole credentials object collapses to the fixed redactionMarker on
//     every object-inspection path (String());
//   - any parameters leaf whose key contains a sensitive token (case-insensitive
//     substring, sensitiveKeySubstrings) collapses to the marker;
//   - the decoded error body is deep-walked (redactBody) as belt-and-suspenders —
//     the API only ever echoes field names, never a credential value, but a
//     future server/proxy change must not be able to leak one.
//
// Internal helpers, not part of the public API.

// redactionMarker is the fixed, fleet-wide redaction marker (D9).
const redactionMarker = "[REDACTED]"

// sensitiveKeySubstrings are case-insensitive substrings that mark a key as
// carrying a secret; a key containing any of them has its whole value masked.
var sensitiveKeySubstrings = []string{
	"token", "password", "passwd", "secret", "key", "keyfile",
	"credential", "passphrase", "sas", "sig", "signature",
}

// isSensitiveKey reports whether a key name marks its value as sensitive
// (case-insensitive substring match).
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, needle := range sensitiveKeySubstrings {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// redactSensitive deep-walks a decoded map and masks the value of every sensitive
// key (isSensitiveKey) to redactionMarker, recursing into nested maps and slices.
// Non-secret keys (bucket, host, file, container, projectid, …) pass through
// untouched. A nil map yields a fresh empty map (never nil), so a rendered
// descriptor shows {} rather than null.
func redactSensitive(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSensitiveKey(k) {
			out[k] = redactionMarker
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return redactSensitive(t)
	case []any:
		arr := make([]any, len(t))
		for i, e := range t {
			arr[i] = redactValue(e)
		}
		return arr
	default:
		return v
	}
}

// redactBody deep-walks a decoded error body and masks every sensitive key
// (including a flattened/dotted key like input.0.credentials.secretaccesskey) to
// the marker before it lands on a typed error. It is the same walk as
// redactSensitive; named separately to mark the belt-and-suspenders intent.
func redactBody(body map[string]any) map[string]any {
	return redactSensitive(body)
}

// redactedParamsJSON renders a parameters map as JSON with sensitive leaves
// masked — safe to embed in an object's String(). A marshalling failure degrades
// to the bare marker rather than risking a raw dump.
func redactedParamsJSON(params map[string]any) string {
	b, err := json.Marshal(redactSensitive(params))
	if err != nil {
		return redactionMarker
	}
	return string(b)
}
