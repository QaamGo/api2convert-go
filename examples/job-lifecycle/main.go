// Command joblifecycle mirrors the "Job Lifecycle" guide: drive the steps by hand
// — create a staged job, attach a remote input, start it, wait, list outputs.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/job-lifecycle
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteJPG = "https://example-files.online-convert.com/raster%20image/jpg/example.jpg"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	jobs := client.Jobs()

	// Stage a job (process:false) so we can attach inputs before starting.
	job, err := jobs.Create(ctx, map[string]any{
		"process":    false,
		"conversion": []any{map[string]any{"category": "image", "target": "png"}},
	})
	if err != nil {
		log.Fatalf("create: %v", err)
	}
	fmt.Printf("created staged job %s\n", job.ID)

	// Attach a remote input, then start processing.
	if _, err := jobs.AddInput(ctx, job.ID, map[string]any{"type": "remote", "source": remoteJPG}); err != nil {
		log.Fatalf("add input: %v", err)
	}
	if _, err := jobs.Start(ctx, job.ID); err != nil {
		log.Fatalf("start: %v", err)
	}

	// Poll to a terminal status (0 = default poll timeout).
	done, err := jobs.Wait(ctx, job.ID, 0, true)
	if err != nil {
		log.Fatalf("wait: %v", err)
	}

	outputs, err := jobs.Outputs(ctx, done.ID)
	if err != nil {
		log.Fatalf("outputs: %v", err)
	}
	fmt.Printf("job %s: %s, %d output(s)\n", done.ID, done.Status.Code, len(outputs))
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
