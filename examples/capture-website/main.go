// Command capturewebsite mirrors the "Capture a Website" guide: screenshot a URL by
// giving the job a remote input with the "screenshot" engine.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/capture-website
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	jobs := client.Jobs()

	job, err := jobs.Create(ctx, map[string]any{
		"process": true,
		"input": []any{map[string]any{
			"type":   "remote",
			"source": "https://www.online-convert.com",
			"engine": "screenshot",
			"options": map[string]any{
				"screen_width":        1280,
				"screen_height":       1024,
				"device_scale_factor": 1,
			},
		}},
		"conversion": []any{map[string]any{"category": "image", "target": "png"}},
	})
	if err != nil {
		log.Fatalf("create: %v", err)
	}

	done, err := jobs.Wait(ctx, job.ID, 0, true)
	if err != nil {
		log.Fatalf("wait: %v", err)
	}
	fmt.Printf("job %s: %s, %d output(s)\n", done.ID, done.Status.Code, len(done.Output))
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
