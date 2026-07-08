// Command presets mirrors the "Presets" guide: list saved conversion presets,
// optionally filtered by category and target.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/presets
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// List presets for video -> mp4 (empty is fine — the account may have none).
	presets, err := client.Presets().List(ctx, "video", "mp4", "")
	if err != nil {
		log.Fatalf("list presets: %v", err)
	}
	fmt.Printf("%d preset(s) for video/mp4\n", len(presets))
	for _, p := range presets {
		fmt.Printf("  %s -> %s\n", p.Name, p.Target)
	}
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
