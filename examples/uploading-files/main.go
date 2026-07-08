// Command uploadingfiles mirrors the "Uploading Files" guide: convert a LOCAL file
// in one call — the SDK stages the job, streams the upload, starts and polls it.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/uploading-files
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

// onePxPNG is a minimal valid 1x1 PNG, written to disk to exercise the real
// multipart upload path (remote-URL inputs skip upload entirely).
var onePxPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// Write a small local file to upload.
	src := filepath.Join(os.TempDir(), "pixel.png")
	if err := os.WriteFile(src, onePxPNG, 0o644); err != nil {
		log.Fatalf("write source: %v", err)
	}

	// One-call upload + convert: pass the local path to Convert.
	res, err := client.Convert(ctx, src, "png")
	if err != nil {
		log.Fatalf("convert: %v", err)
	}
	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "uploaded.png"))
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
