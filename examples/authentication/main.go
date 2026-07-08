// Command authentication mirrors the "Authentication" guide: prove the API key
// works by making an authenticated call — list the account's jobs.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/authentication
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	// New reads the key from API2CONVERT_API_KEY when "" is passed.
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// An authenticated request — succeeds only when the key is valid.
	jobs, err := client.Jobs().List(ctx, "", 1)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}
	fmt.Printf("authenticated: %d job(s) on page 1\n", len(jobs))
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
