package api2convert

import "fmt"

// OutputTarget is a cloud-storage delivery target for a conversion's output:
// { type:<provider>, parameters, credentials }.
//
// Attach one (or more) to a conversion via Client.Convert / Client.ConvertAsync
// (the WithOutputTarget / WithOutputTargets controls), or inline in a raw
// Jobs().Create conversion map. When any output target is set the conversion
// delivers straight to your storage and produces no local output — so Convert
// returns the completed job without downloading.
//
// This wave ships the generic shape only (type + free-form parameters/
// credentials); the per-provider output keys live in a separate service and
// diverge per provider, so there are no per-provider output factories yet.
//
// Descriptor emits { type, parameters, credentials } and omits status (server-set,
// read-only). On read (OutputTargetFromMap) type, parameters and status round-trip
// as raw values; credentials are never surfaced (the API returns them empty).
// credentials ride in the plaintext body, so String() masks the whole object to
// [REDACTED].
type OutputTarget struct {
	// Type is the provider string (a CloudProvider value, kept as a raw string).
	Type string
	// Parameters are delivery locator keys (provider-specific).
	Parameters map[string]any
	// Credentials are secret keys (never surfaced on read).
	Credentials map[string]any
	// Status is the server-set delivery status on read
	// (waiting|uploading|completed|failed); never sent on create (empty means absent).
	Status string
}

// OutputTargetOf builds a generic output target for a typed provider or a
// forward-compat CloudProvider("...") value. status is server-set and left empty.
func OutputTargetOf(targetType CloudProvider, parameters, credentials map[string]any) OutputTarget {
	return OutputTarget{Type: string(targetType), Parameters: parameters, Credentials: credentials}
}

// Descriptor is the wire descriptor sent on create — { type, parameters,
// credentials } — with status omitted (server-set, read-only). Nil maps normalize
// to empty objects.
func (o OutputTarget) Descriptor() map[string]any {
	return map[string]any{
		"type":        o.Type,
		"parameters":  nonNilMap(o.Parameters),
		"credentials": nonNilMap(o.Credentials),
	}
}

// OutputTargetFromMap hydrates from a GET /jobs/{id} output_target[] element.
// type/status stay raw strings (an unknown provider round-trips untyped);
// credentials are deliberately not surfaced.
func OutputTargetFromMap(data map[string]any) OutputTarget {
	return OutputTarget{
		Type:        asString(data["type"], ""),
		Parameters:  asObject(data["parameters"]),
		Credentials: map[string]any{},
		Status:      asString(data["status"], ""),
	}
}

// String is a human-readable form with credentials masked — safe to log.
func (o OutputTarget) String() string {
	status := o.Status
	if status == "" {
		status = "null"
	}
	return fmt.Sprintf(
		"OutputTarget(type=%s, parameters=%s, credentials=%s, status=%s)",
		o.Type, redactedParamsJSON(o.Parameters), redactionMarker, status,
	)
}
