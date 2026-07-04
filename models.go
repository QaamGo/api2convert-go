package api2convert

// Status is a job's status: a machine-readable Code plus optional human Info.
type Status struct {
	Code string
	Info string
}

// StatusFromMap hydrates a Status from a decoded JSON object.
func StatusFromMap(data map[string]any) Status {
	return Status{
		Code: asString(data["code"], ""),
		Info: asString(data["info"], ""),
	}
}

// Conversion is a single conversion within a job: the target format plus options.
type Conversion struct {
	Target   string
	ID       string
	Category string
	Options  map[string]any
	Metadata map[string]any
}

// ConversionFromMap hydrates a Conversion from a decoded JSON object.
func ConversionFromMap(data map[string]any) Conversion {
	return Conversion{
		Target:   asString(data["target"], ""),
		ID:       asString(data["id"], ""),
		Category: asString(data["category"], ""),
		Options:  asObject(data["options"]),
		Metadata: asObject(data["metadata"]),
	}
}

// InputFile is an input file attached to a job.
type InputFile struct {
	ID          string
	Type        string
	Source      string
	Status      string
	Filename    string
	Size        *int64
	ContentType string
	Options     map[string]any
}

// InputFileFromMap hydrates an InputFile from a decoded JSON object.
func InputFileFromMap(data map[string]any) InputFile {
	return InputFile{
		ID:          asString(data["id"], ""),
		Type:        asString(data["type"], ""),
		Source:      asString(data["source"], ""),
		Status:      asString(data["status"], ""),
		Filename:    asString(data["filename"], ""),
		Size:        nullableInt64(data["size"]),
		ContentType: asString(data["content_type"], ""),
		Options:     asObject(data["options"]),
	}
}

// OutputFile is a produced output file. URI is a self-contained download URL (no
// auth required), valid for a limited time (24h by default).
type OutputFile struct {
	ID          string
	URI         string
	Filename    string
	Size        *int64
	Status      string
	ContentType string
	Checksum    string
	Metadata    map[string]any
}

// OutputFileFromMap hydrates an OutputFile from a decoded JSON object.
func OutputFileFromMap(data map[string]any) OutputFile {
	return OutputFile{
		ID:          asString(data["id"], ""),
		URI:         asString(data["uri"], ""),
		Filename:    asString(data["filename"], ""),
		Size:        nullableInt64(data["size"]),
		Status:      asString(data["status"], ""),
		ContentType: asString(data["content_type"], ""),
		Checksum:    asString(data["checksum"], ""),
		Metadata:    asObject(data["metadata"]),
	}
}

// OutputFileOf constructs an OutputFile from its essentials (mirrors the siblings'
// OutputFile.of).
func OutputFileOf(id, uri, filename string) OutputFile {
	return OutputFile{ID: id, URI: uri, Filename: filename}
}

// Preset is a saved conversion preset (a reusable named target + options).
type Preset struct {
	ID       string
	Name     string
	Target   string
	Category string
	Scope    string
	Options  map[string]any
}

// PresetFromMap hydrates a Preset from a decoded JSON object.
func PresetFromMap(data map[string]any) Preset {
	return Preset{
		ID:       asString(data["id"], ""),
		Name:     asString(data["name"], ""),
		Target:   asString(data["target"], ""),
		Category: asString(data["category"], ""),
		Scope:    asString(data["scope"], ""),
		Options:  asObject(data["options"]),
	}
}

// JobMessage is an error or warning attached to a job (the errors[] / warnings[]
// entries).
type JobMessage struct {
	Code     *int
	Message  string
	Source   string
	IDSource string
	Details  map[string]any
}

// JobMessageFromMap hydrates a JobMessage from a decoded JSON object.
func JobMessageFromMap(data map[string]any) JobMessage {
	var code *int
	if n := nullableInt64(data["code"]); n != nil {
		c := int(*n)
		code = &c
	}
	return JobMessage{
		Code:     code,
		Message:  asString(data["message"], ""),
		Source:   asString(data["source"], ""),
		IDSource: asString(data["id_source"], ""),
		Details:  asObject(data["details"]),
	}
}
