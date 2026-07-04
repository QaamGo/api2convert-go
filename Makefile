GO ?= go
STATICCHECK_VERSION ?= latest

.PHONY: help fmt fmt-fix vet staticcheck test test-security test-live check tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

fmt: ## Fail if any file is not gofmt-clean
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

fmt-fix: ## Rewrite files with gofmt
	gofmt -w .

vet: ## Run go vet
	$(GO) vet ./...

staticcheck: ## Run staticcheck (pinned; never added to go.mod)
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

test: ## Offline unit tests + the hermetic security suite (no API key needed)
	$(GO) test -race ./...

test-security: ## The independent security suite, run in isolation
	$(GO) test -race -v ./security/...

test-live: ## Live conformance — requires API2CONVERT_API_KEY (export the behat key)
	$(GO) test -tags live -v -timeout 300s ./live/...

check: fmt vet test test-security ## Default guardrail (fmt + vet + unit + security)

tidy: ## Verify go.mod stays dependency-free
	$(GO) mod tidy
	@if [ -n "$$(git status --porcelain go.mod go.sum 2>/dev/null)" ]; then \
		echo "go.mod/go.sum changed — the SDK must stay dependency-free"; exit 1; fi
