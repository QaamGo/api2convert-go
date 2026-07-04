package api2convert

import "fmt"

// The typed error hierarchy.
//
// Every failure the SDK returns satisfies Api2ConvertError. HTTP error responses
// (status >= 400) additionally satisfy HTTPError and map to a dedicated concrete
// type per status; transport failures, conversion failures, poll timeouts and
// webhook verification failures descend directly from the base.
//
// Match failures with errors.As:
//
//	var rl *api2convert.RateLimitError
//	if errors.As(err, &rl) { ... rl.RetryAfter ... }
//
//	var he api2convert.HTTPError
//	if errors.As(err, &he) { ... he.Status() ... }   // any HTTP error
//
//	var any api2convert.Api2ConvertError
//	if errors.As(err, &any) { ... }                   // any SDK error
//
// Secrets (API key, upload token, download password) are never placed in any
// error message.

// Api2ConvertError is satisfied by every error the SDK returns. Use it as the
// broadest catch.
type Api2ConvertError interface {
	error
	isApi2ConvertError()
}

// HTTPError is satisfied by every error that originated from an HTTP error
// response (status >= 400). It exposes the status code, request id and decoded
// body.
type HTTPError interface {
	Api2ConvertError
	Status() int
	RequestID() string
	Body() map[string]any
}

// genericError is the shared implementation embedded by every SDK error.
type genericError struct {
	Message string
	Cause   error
}

func (e *genericError) Error() string       { return e.Message }
func (e *genericError) Unwrap() error       { return e.Cause }
func (e *genericError) isApi2ConvertError() {}

// apiErrorBase carries the HTTP fields shared by every status-mapped error.
type apiErrorBase struct {
	genericError
	StatusCode int
	RequestID_ string
	Body_      map[string]any
}

func (e *apiErrorBase) Status() int          { return e.StatusCode }
func (e *apiErrorBase) RequestID() string    { return e.RequestID_ }
func (e *apiErrorBase) Body() map[string]any { return e.Body_ }

// ConfigError signals a client that was constructed with invalid configuration
// (for example, no API key). It is returned by New.
type ConfigError struct{ genericError }

// APIError is an HTTP error response (status >= 400) with no more specific type —
// a 4xx the SDK does not map to a dedicated class.
type APIError struct{ apiErrorBase }

// AuthenticationError means the API key is missing, invalid or not permitted
// (HTTP 401 / 403).
type AuthenticationError struct{ apiErrorBase }

// PaymentRequiredError means the account has no remaining quota/credit (HTTP 402).
type PaymentRequiredError struct{ apiErrorBase }

// NotFoundError means the requested resource does not exist (HTTP 404).
type NotFoundError struct{ apiErrorBase }

// ValidationError means the request was rejected as invalid, e.g. an unknown
// target (HTTP 400 / 422).
type ValidationError struct{ apiErrorBase }

// RateLimitError means too many requests (HTTP 429); returned only once
// auto-retries are exhausted.
type RateLimitError struct {
	apiErrorBase
	// RetryAfter is the seconds to wait before retrying, parsed from the
	// Retry-After header (raw, uncapped); nil when the header was absent.
	RetryAfter *int
}

// ServerError is a server-side error (HTTP >= 500), returned once auto-retries
// are exhausted.
type ServerError struct{ apiErrorBase }

// NetworkError means a request did not yield a usable response: a transport-level
// failure (DNS/connection/TLS/read) once idempotent retries are exhausted, a 2xx
// whose body is not valid JSON, or a malformed API-supplied URL.
type NetworkError struct{ genericError }

// ConversionFailedError means a job reached the failed (or canceled) status. The
// originating Job is attached so you can inspect its Errors and Warnings.
type ConversionFailedError struct {
	genericError
	Job Job
}

// Errors returns the failed job's errors (may be empty if the API gave no detail).
func (e *ConversionFailedError) Errors() []JobMessage { return e.Job.Errors }

// ConversionTimeoutError means a job did not reach a terminal status within the
// configured poll timeout. The job is still running server-side — re-fetch it
// later with client.Jobs().Get(ctx, job.ID).
type ConversionTimeoutError struct {
	genericError
	Job Job
}

// SignatureVerificationError means a webhook payload could not be verified against
// the provided signature/secret. Treat it as a security event: do not trust the
// payload.
type SignatureVerificationError struct{ genericError }

// newError builds a base SDK error.
func newError(message string, cause error) *genericError {
	return &genericError{Message: message, Cause: cause}
}

// newAPIError maps an HTTP error response to the appropriate typed error.
func newAPIError(status int, message, requestID string, body map[string]any, retryAfter *int) error {
	base := apiErrorBase{
		genericError: genericError{Message: message},
		StatusCode:   status,
		RequestID_:   requestID,
		Body_:        body,
	}
	switch {
	case status == 401 || status == 403:
		return &AuthenticationError{base}
	case status == 402:
		return &PaymentRequiredError{base}
	case status == 404:
		return &NotFoundError{base}
	case status == 429:
		return &RateLimitError{apiErrorBase: base, RetryAfter: retryAfter}
	case status == 400 || status == 422:
		return &ValidationError{base}
	case status >= 500:
		return &ServerError{base}
	default:
		return &APIError{base}
	}
}

func newConversionFailedError(job Job) *ConversionFailedError {
	return &ConversionFailedError{genericError: genericError{Message: conversionFailedMessage(job)}, Job: job}
}

func conversionFailedMessage(job Job) string {
	if len(job.Errors) > 0 {
		first := job.Errors[0]
		if first.Code != nil {
			return fmt.Sprintf("Conversion failed: %s (code %d)", first.Message, *first.Code)
		}
		return "Conversion failed: " + first.Message
	}
	if job.Status.Info != "" {
		return "Conversion failed: " + job.Status.Info
	}
	return "Conversion failed."
}

func newConversionTimeoutError(job Job, timeoutSeconds float64) *ConversionTimeoutError {
	return &ConversionTimeoutError{
		genericError: genericError{Message: fmt.Sprintf(
			"Timed out after %gs waiting for job %s to finish (last status: %s).",
			timeoutSeconds, job.ID, job.Status.Code)},
		Job: job,
	}
}
