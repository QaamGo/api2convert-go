// Command convert demonstrates the one-call conversion flow.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/convert photo.png jpg out/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: %s <input-path-or-url> <target-format> <output-path-or-dir>", os.Args[0])
	}
	input, target, out := os.Args[1], os.Args[2], os.Args[3]

	// The API key is read from the API2CONVERT_API_KEY environment variable.
	client, err := api2convert.New("")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	res, err := client.Convert(ctx, input, target,
		api2convert.WithConversionOptions(map[string]any{"quality": 85}),
	)
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	path, err := res.Save(ctx, out)
	if err != nil {
		log.Fatalf("save: %v", err)
	}
	fmt.Printf("saved %s\n", path)
}
