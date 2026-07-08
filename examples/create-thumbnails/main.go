// Command createthumbnails mirrors the "Create Thumbnails" guide: render a preview
// image of a document page via the "thumbnail" operation.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/create-thumbnails
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remotePDF = "https://example-files.online-convert.com/document/pdf/example.pdf"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	res, err := client.Convert(ctx, remotePDF, "thumbnail",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{
			"thumbnail_target": "png",
			"width":            300,
			"pages":            "first",
			"dpi":              150,
		}))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "thumbnail.png"))
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
