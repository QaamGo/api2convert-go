// Command audiooperations mirrors the "Audio Operations" guide: transcode audio to
// AAC with explicit codec, bitrate, channel and frequency options.
//
//	API2CONVERT_API_KEY=<key> go run ./examples/audio-operations
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteWAV = "https://example-files.online-convert.com/audio/wav/example.wav"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	res, err := client.Convert(ctx, remoteWAV, "aac",
		api2convert.WithCategory("audio"),
		api2convert.WithConversionOptions(map[string]any{
			"audio_codec":   "aac",
			"audio_bitrate": 192,
			"channels":      "stereo",
			"frequency":     44100,
		}))
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	path, err := res.Save(ctx, filepath.Join(os.TempDir(), "audio.aac"))
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
