// Command statistics mirrors the "Statistics" guide: fetch usage figures for a
// month via the stats resource.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/statistics
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// Usage for the current month (format yyyy-mm), across all keys ("all").
	month := time.Now().UTC().Format("2006-01")
	stats, err := client.Stats().Month(ctx, month, "all")
	if err != nil {
		log.Fatalf("stats: %v", err)
	}
	fmt.Printf("usage for %s: %v\n", month, stats)
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
