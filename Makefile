BINARY  := bin/hop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test e2e lint clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/hop

test:
	go vet ./...
	go test -race ./...

e2e: build
	go test -race -tags=integration ./test/...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin
