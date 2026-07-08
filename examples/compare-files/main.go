// Command comparefiles mirrors the "Compare Files" guide: diff two images with the
// "compare-image" operation and produce a visual difference map.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/compare-files
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const (
	remoteJPG      = "https://example-files.online-convert.com/raster%20image/jpg/example.jpg"
	remoteJPGSmall = "https://example-files.online-convert.com/raster%20image/jpg/example_small.jpg"
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
		"input": []any{
			map[string]any{"type": "remote", "source": remoteJPGSmall},
			map[string]any{"type": "remote", "source": remoteJPG},
		},
		"conversion": []any{map[string]any{
			"category": "operation",
			"target":   "compare-image",
			"options":  map[string]any{"method": "ssim", "threshold": 5, "diff_color": "red"},
		}},
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
