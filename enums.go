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
