// Command webhook demonstrates verifying an API2Convert webhook callback inside an
// HTTP handler.
//
//	API2CONVERT_WEBHOOK_SECRET=<secret> go run ./examples/webhook
package main

import (
	"io"
	"log"
	"net/http"
	"os"

	api2convert "github.com/QaamGo/api2convert-go/v10"
)

func main() {
	secret := os.Getenv("API2CONVERT_WEBHOOK_SECRET") // empty secret skips verification

	http.HandleFunc("/webhooks/api2convert", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		// Pass the RAW body so signature verification is byte-exact.
		event, err := api2convert.Webhooks().ConstructEvent(body, r.Header.Get("X-Oc-Signature"), secret)
		if err != nil {
			// Treat a verification failure as a security event: do not process it.
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		job := event.Job
		log.Printf("job %s is now %s (completed=%v)", job.ID, job.Status.Code, job.IsCompleted())
		w.WriteHeader(http.StatusNoContent)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
