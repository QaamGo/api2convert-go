package api2convert

// Job is a conversion job — the central API2Convert resource.
//
// Server and Token are needed to upload local files; Output holds the produced
// files once the job IsCompleted. Raw keeps the full decoded response for fields
// not surfaced as typed properties.
//
// Nullable string fields from the API are represented as plain strings where the
// empty string means "absent"; consult Raw for the exact JSON value if the
// distinction matters.
type Job struct {
	ID         string
	Status     Status
	Token      string
	Server     string
	Callback   string
	Conversion []Conversion
	Input      []InputFile
	Output     []OutputFile
	Errors     []JobMessage
	Warnings   []JobMessage
	Raw        map[string]any
}

// IsCompleted reports whether the job finished successfully (status.code == "completed").
func (j Job) IsCompleted() bool { return j.Status.Code == string(JobStatusCompleted) }

// IsFailed reports whether the job finished unsuccessfully (status.code == "failed").
func (j Job) IsFailed() bool { return j.Status.Code == string(JobStatusFailed) }

// IsCanceled reports whether the job was canceled server-side (terminal, no output).
func (j Job) IsCanceled() bool { return j.Status.Code == string(JobStatusCanceled) }

// IsTerminal reports whether the job finished (completed, failed or canceled) and
// will not change further.
func (j Job) IsTerminal() bool { return IsTerminalCode(j.Status.Code) }

// JobFromMap hydrates a Job from a decoded JSON object. It never panics on a
// surprising payload; missing or wrong-typed fields fall back to zero values.
func JobFromMap(data map[string]any) Job {
	return Job{
		ID:         asString(data["id"], ""),
		Status:     StatusFromMap(asObject(data["status"])),
		Token:      asString(data["token"], ""),
		Server:     asString(data["server"], ""),
		Callback:   asString(data["callback"], ""),
		Conversion: mapObjects(data["conversion"], ConversionFromMap),
		Input:      mapObjects(data["input"], InputFileFromMap),
		Output:     mapObjects(data["output"], OutputFileFromMap),
		Errors:     mapObjects(data["errors"], JobMessageFromMap),
		Warnings:   mapObjects(data["warnings"], JobMessageFromMap),
		Raw:        data,
	}
}
