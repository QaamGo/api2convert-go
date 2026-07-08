// Command createhashes mirrors the "Create Hashes" guide: compute a SHA-256 digest
// of a file via the "sha256" hash target.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/create-hashes
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteZIP = "https://example-files.online-convert.com/archive/zip/example.zip"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	res, err := client.Convert(ctx, remoteZIP, "sha256", api2convert.WithCategory("hash"))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	// The hash result is a small file whose contents are the digest.
	contents, err := res.Contents(ctx)
	if err != nil {
		log.Fatalf("download: %v", err)
	}
	fmt.Printf("sha256: %s\n", string(contents))
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
