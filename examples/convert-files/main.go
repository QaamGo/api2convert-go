// Command convertfiles mirrors the "Convert Files" guide: browse the conversions
// catalog (all, then filtered to a target), then run a conversion.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/convert-files
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

	// The full catalog (first page) — the source of truth for supported targets.
	all, err := client.Conversions().List(ctx, "", "", 1)
	if err != nil {
		log.Fatalf("list catalog: %v", err)
	}
	fmt.Printf("catalog: %d conversions on page 1\n", len(all))

	// Filter the catalog to conversions that target PNG.
	toPNG, err := client.Conversions().List(ctx, "", "png", 1)
	if err != nil {
		log.Fatalf("list png conversions: %v", err)
	}
	fmt.Printf("conversions to png: %d\n", len(toPNG))

	// Convert a JPG to PNG.
	res, err := client.Convert(ctx, remoteJPG, "png")
	if err != nil {
		log.Fatalf("convert: %v", err)
	}
	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "convert-files.png"))
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
