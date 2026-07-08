// Command compressfiles mirrors the "Compress Files" guide: shrink a file with the
// "compress" operation.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/compress-files
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

	res, err := client.Convert(ctx, remoteJPG, "compress",
		api2convert.WithCategory("operation"),
		api2convert.WithConversionOptions(map[string]any{"compression_level": "high"}))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "compressed.jpg"))
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
