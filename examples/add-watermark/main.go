// Command addwatermark mirrors the "Add a Watermark" guide: stamp a PDF with an
// image overlay by giving the job two remote inputs (the document and the stamp).
//
//	API2CONVERT_API_KEY=<key> go run ./examples/add-watermark
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const (
	remotePDF = "https://example-files.online-convert.com/document/pdf/example.pdf"
	remotePNG = "https://example-files.online-convert.com/raster%20image/png/example.png"
)

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	jobs := client.Jobs()

	// The document is the primary input; the image is the stamp overlay.
	job, err := jobs.Create(ctx, map[string]any{
		"process": true,
		"input": []any{
			map[string]any{"type": "remote", "source": remotePDF},
			map[string]any{"type": "remote", "source": remotePNG},
		},
		"conversion": []any{map[string]any{
			"category": "document",
			"target":   "pdf",
			"options":  map[string]any{"stamp": true, "alignment": "center"},
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
