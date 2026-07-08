// Command quickstart mirrors the "Quickstart" guide: convert a remote JPG to PNG,
// fetch the finished job by id, then download the output.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/quickstart
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteJPG = "https://example-files.online-convert.com/raster%20image/jpg/example.jpg"

func main() {
	// The API key is read from API2CONVERT_API_KEY; API2CONVERT_BASE_URL is
	// honored when set (e.g. to target a beta environment).
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Convert a remote image to PNG (the SDK creates the job, polls it to
	// completion, and hands back a result you can download).
	res, err := client.Convert(ctx, remoteJPG, "png")
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	// Re-fetch the finished job by id.
	job, err := client.Jobs().Get(ctx, res.Job.ID)
	if err != nil {
		log.Fatalf("get job: %v", err)
	}
	fmt.Printf("job %s: %s\n", job.ID, job.Status.Code)

	// Download the output.
	out := filepath.Join(os.TempDir(), "quickstart.png")
	path, err := res.Save(ctx, out)
	if err != nil {
		log.Fatalf("save: %v", err)
	}
	fmt.Printf("saved %s\n", path)
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
