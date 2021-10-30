GO_FLAGS=CGO_ENABLED=0

check-cgo-free:
	$(GO_FLAGS) go build ./...
.PHONY: check-cgo-free

test:
	go test ./... -race
.PHONY: test

