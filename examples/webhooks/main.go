// Command webhooks mirrors the "Webhooks" guide. It has two halves:
//
//   - Sending: start an async conversion with a callback URL. The API POSTs a
//     status update to that URL when the job finishes (see ConvertAsync below).
//
//   - Receiving: verify the signed callback with api2convert.Webhooks() (see
//     handleWebhook, wired up for reference).
//
//     API2CONVERT_API_KEY=<key> go run ./examples/webhooks
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

const remoteDOCX = "https://example-files.online-convert.com/document/docx/example.docx"

func main() {
	client, err := api2convert.New("", baseURLOpts()...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// Start the conversion asynchronously with a callback URL. This returns as
	// soon as the job is started — the API notifies the callback on completion.
	job, err := client.ConvertAsync(ctx, remoteDOCX, "pdf",
		api2convert.WithCategory("document"),
		api2convert.WithCallback("https://your-app.example.com/api2convert/webhook"))
	if err != nil {
		log.Fatalf("convert async: %v", err)
	}
	fmt.Printf("started job %s (status %s); a webhook will fire on completion\n", job.ID, job.Status.Code)

	// The receiving side (not started here) verifies each callback:
	_ = handleWebhook
}

// handleWebhook verifies an incoming API2Convert callback. Pass the RAW request
// body so the HMAC-SHA256 signature check is byte-exact.
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("API2CONVERT_WEBHOOK_SECRET") // empty secret skips verification
	// Bound the read: a webhook body is small, and an unbounded ReadAll on an
	// attacker-controlled request is a memory-exhaustion vector.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	event, err := api2convert.Webhooks().ConstructEvent(body, r.Header.Get("X-Oc-Signature"), secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	log.Printf("job %s is now %s", event.Job.ID, event.Job.Status.Code)
	w.WriteHeader(http.StatusNoContent)
}

func baseURLOpts() []api2convert.Option {
	if base := os.Getenv("API2CONVERT_BASE_URL"); base != "" {
		return []api2convert.Option{api2convert.WithBaseURL(base)}
	}
	return nil
}
