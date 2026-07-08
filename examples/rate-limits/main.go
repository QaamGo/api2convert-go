// Command ratelimits mirrors the "Rate Limits" guide: inspect the account's active
// contracts (which govern quota and rate limits) via the contracts resource.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/rate-limits
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

	contracts, err := client.Contracts().Get(ctx)
	if err != nil {
		log.Fatalf("contracts: %v", err)
	}
	fmt.Printf("contracts: %v\n", contracts)
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
