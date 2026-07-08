// Command extractassets mirrors the "Extract Assets" guide: pull the embedded
// assets (images, media, ...) out of a document via the "extract-assets" operation.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/extract-assets
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteDOCX = "https://example-files.online-convert.com/document/docx/example.docx"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	res, err := client.Convert(ctx, remoteDOCX, "extract-assets",
		api2convert.WithCategory("operation"))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	outputs := res.Outputs()
	fmt.Printf("job %s: %s, %d asset(s) extracted\n", res.Job.ID, res.Job.Status.Code, len(outputs))
	for _, o := range outputs {
		fmt.Printf("  %s\n", o.Filename)
	}
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
