// Command imageoperations mirrors the "Image Operations" guide: resize an image with
// the "resize-image" operation, keeping the aspect ratio via a crop.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/image-operations
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
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	res, err := client.Convert(ctx, remoteJPG, "resize-image",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{
			"width":           800,
			"height":          600,
			"resize_by":       "px",
			"resize_handling": "keep_aspect_ratio_crop",
		}))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "resized.jpg"))
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
