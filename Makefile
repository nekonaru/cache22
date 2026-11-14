BINARY  := cache22
CLI     := deadrop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet run cli docker clean

all: vet test build

build: ## build server + CLI into bin/
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/cache22
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(CLI) ./cmd/deadrop

test: ## run the full test suite with the race detector
	go test -race ./...

vet: ## static analysis
	go vet ./...

run: ## run the server on :8080
	go run ./cmd/cache22

cli: ## build only the CLI
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(CLI) ./cmd/deadrop

docker: ## build the container image
	docker build -t cache22:$(VERSION) .

clean:
	rm -rf bin dist coverage.*

help: ## list targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'
