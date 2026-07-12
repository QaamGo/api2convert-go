package api2convert

// JobStatus enumerates the well-known job status codes (the status.code field).
//
// The API may introduce further codes; treat any code not listed here as
// non-terminal. Use IsTerminalCode for a raw status string rather than comparing
// by hand.
type JobStatus string

const (
	JobStatusCreated     JobStatus = "created"
	JobStatusIncomplete  JobStatus = "incomplete"
	JobStatusDownloading JobStatus = "downloading"
	JobStatusQueued      JobStatus = "queued"
	JobStatusProcessing  JobStatus = "processing"
	JobStatusCompleted   JobStatus = "completed"
	JobStatusFailed      JobStatus = "failed"
	JobStatusCanceled    JobStatus = "canceled"
)

// IsTerminalCode reports whether a raw status code is terminal (completed, failed
// or canceled). Unknown codes are treated as non-terminal.
func IsTerminalCode(code string) bool {
	switch JobStatus(code) {
	case JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
		return true
	default:
		return false
	}
}

// InputType enumerates the kinds of source an input file can be created from (the
// input "type" field). A typed reference for building input descriptors by hand,
// e.g. AddInput(ctx, jobID, map[string]any{"type": string(InputTypeRemote), "source": "..."}).
type InputType string

const (
	InputTypeUpload       InputType = "upload"
	InputTypeRemote       InputType = "remote"
	InputTypeOutput       InputType = "output"
	InputTypeInputID      InputType = "input_id"
	InputTypeGdrivePicker InputType = "gdrive_picker"
	InputTypeBase64       InputType = "base64"
	InputTypeCloud        InputType = "cloud"
)

// CloudProvider is the vocabulary of cloud storage providers the API can import
// inputs from and deliver outputs to — the value of a cloud descriptor's "source"
// (input) or "type" (output) field.
//
// It is build-side vocabulary only: it types the CloudInput builder and
// OutputTarget serialization. Read models keep source/type/status as raw strings,
// so an unknown provider string returned by the server round-trips untyped and
// never fails to hydrate. Pass a CloudProvider("...") conversion for a
// forward-compat provider the constants do not yet name.
//
// Import support (a CloudInput constructor) exists for CloudProviderAmazonS3,
// CloudProviderAzure, CloudProviderFtp and CloudProviderGoogleCloud.
// CloudProviderGdrive and CloudProviderYoutube are output-only (they validate as
// an output type but have no downloader); Google Drive input uses the separate
// gdrive_picker input type via the raw AddInput path.
type CloudProvider string

const (
	CloudProviderAmazonS3    CloudProvider = "amazons3"
	CloudProviderAzure       CloudProvider = "azure"
	CloudProviderFtp         CloudProvider = "ftp"
	CloudProviderGdrive      CloudProvider = "gdrive"
	CloudProviderGoogleCloud CloudProvider = "googlecloud"
	CloudProviderYoutube     CloudProvider = "youtube"
)
