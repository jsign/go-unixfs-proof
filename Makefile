GO_FLAGS=CGO_ENABLED=0

check-cgo-free:
	$(GO_FLAGS) go build ./...
.PHONY: check-cgo-free

test:
	go test ./... -race
.PHONY: test

GOLANGCI_LINT=go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.1
lint:
	$(GOLANGCI_LINT) run
.PHONY: lint
	
