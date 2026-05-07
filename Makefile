BINARY   := inode
MODULE   := github.com/shahid-io/inode
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -s -w \
            -X $(MODULE)/internal/version.Version=$(VERSION) \
            -X $(MODULE)/internal/version.Commit=$(COMMIT) \
            -X $(MODULE)/internal/version.Date=$(DATE)

INSTALL_DIR := /opt/homebrew/bin

.PHONY: build install run test lint clean tidy help

build:                         ## Build binary for current platform
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: build                 ## Build and install to $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed → $(INSTALL_DIR)/$(BINARY)"

run:                           ## Build and run with args (usage: make run ARGS="ask 'query'")
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

test:                          ## Run all tests with race detector
	go test -race -cover ./...

lint:                          ## Run golangci-lint
	golangci-lint run ./...

tidy:                          ## Tidy and verify go modules
	go mod tidy
	go mod verify

clean:                         ## Remove build artifacts
	rm -f $(BINARY)
	rm -f coverage.out

help:                          ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
