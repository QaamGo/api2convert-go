// Command fileanalysis mirrors the "File Analysis" guide: extract a file's metadata
// as JSON via the "json" metadata target.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/file-analysis
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

	res, err := client.Convert(ctx, remoteJPG, "json", api2convert.WithCategory("metadata"))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	// The metadata output is a JSON document describing the input file.
	contents, err := res.Contents(ctx)
	if err != nil {
		log.Fatalf("download: %v", err)
	}
	fmt.Printf("metadata (%d bytes):\n%s\n", len(contents), string(contents))
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
