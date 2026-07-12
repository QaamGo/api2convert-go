package api2convert

import "fmt"

// CloudInput is a cloud-storage input descriptor:
// { type:"cloud", source:<provider>, parameters, credentials }.
//
// Hand it to Client.Convert / Client.ConvertAsync as the input, or to
// Client.Jobs().AddInput(ctx, jobID, in.Descriptor()); either way it emits the
// wire descriptor via Descriptor(). Like a remote URL, a cloud input is a started
// job (process:true), not a staged upload.
//
// The per-provider constructors carry each provider's required keys verbatim —
// flat and lowercase, exactly as the API expects (accesskeyid, not
// access_key_id). Those required keys are constructor arguments (structural
// correctness), not a runtime gate: the builder never rejects a descriptor the
// permissive, asynchronously-validating server would accept. Optional and
// forward-compat keys go through the generic CloudInputOf escape hatch.
//
// Google Drive input uses the gdrive_picker input type (the generic AddInput
// raw-map path this wave); gdrive/youtube are output-only.
//
// credentials ride in the plaintext body, so String() masks the whole
// credentials object to [REDACTED] and any sensitive parameters leaf.
type CloudInput struct {
	// Source is the provider string (a CloudProvider value, kept as a raw string).
	Source string
	// Parameters are non-secret locator keys (bucket, file, host, …).
	Parameters map[string]any
	// Credentials are secret keys (access keys, passwords, tokens).
	Credentials map[string]any
}

// CloudInputOf is the generic escape hatch: any provider (a typed CloudProvider
// or a forward-compat CloudProvider("...") value) with free-form maps.
func CloudInputOf(source CloudProvider, parameters, credentials map[string]any) CloudInput {
	return CloudInput{Source: string(source), Parameters: parameters, Credentials: credentials}
}

// CloudInputAmazonS3 imports from Amazon S3.
func CloudInputAmazonS3(bucket, file, accesskeyid, secretaccesskey string) CloudInput {
	return CloudInput{
		Source:      string(CloudProviderAmazonS3),
		Parameters:  map[string]any{"bucket": bucket, "file": file},
		Credentials: map[string]any{"accesskeyid": accesskeyid, "secretaccesskey": secretaccesskey},
	}
}

// CloudInputAzure imports from Azure Blob Storage.
func CloudInputAzure(container, file, accountname, accountkey string) CloudInput {
	return CloudInput{
		Source:      string(CloudProviderAzure),
		Parameters:  map[string]any{"container": container, "file": file},
		Credentials: map[string]any{"accountname": accountname, "accountkey": accountkey},
	}
}

// CloudInputFTP imports from an FTP server.
func CloudInputFTP(host, file, username, password string) CloudInput {
	return CloudInput{
		Source:      string(CloudProviderFtp),
		Parameters:  map[string]any{"host": host, "file": file},
		Credentials: map[string]any{"username": username, "password": password},
	}
}

// CloudInputGoogleCloud imports from Google Cloud Storage.
func CloudInputGoogleCloud(projectid, bucket, file, keyfile string) CloudInput {
	return CloudInput{
		Source:      string(CloudProviderGoogleCloud),
		Parameters:  map[string]any{"projectid": projectid, "bucket": bucket, "file": file},
		Credentials: map[string]any{"keyfile": keyfile},
	}
}

// Descriptor is the wire descriptor sent to POST /jobs (inline input) or
// POST /jobs/{id}/input. Nil maps normalize to empty objects so the payload keys
// are always present.
func (c CloudInput) Descriptor() map[string]any {
	return map[string]any{
		"type":        string(InputTypeCloud),
		"source":      c.Source,
		"parameters":  nonNilMap(c.Parameters),
		"credentials": nonNilMap(c.Credentials),
	}
}

// String is a human-readable form with credentials masked — safe to log. The
// whole credentials object renders as [REDACTED]; sensitive parameters leaves are
// masked too.
func (c CloudInput) String() string {
	return fmt.Sprintf(
		"CloudInput(type=cloud, source=%s, parameters=%s, credentials=%s)",
		c.Source, redactedParamsJSON(c.Parameters), redactionMarker,
	)
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
