.PHONY: all build clean proto run-backend run-proxy run demo test lint help

# Build configuration
BINARY_DIR := bin
PROXY_BIN := $(BINARY_DIR)/sentinel-proxy
BACKEND_BIN := $(BINARY_DIR)/sentinel-backend
GO := go
GOFLAGS := -trimpath -ldflags="-s -w"

all: build ## Build all binaries

build: $(PROXY_BIN) $(BACKEND_BIN) ## Build proxy and backend

$(PROXY_BIN): cmd/proxy/main.go $(shell find internal -name '*.go')
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/proxy

$(BACKEND_BIN): cmd/backend/main.go $(shell find internal -name '*.go')
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/backend

clean: 
	rm -rf $(BINARY_DIR)
	rm -f backend

proto: ## Regenerate protobuf code
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/sentinel/sentinel.proto

run-backend: $(BACKEND_BIN) ## Run the gRPC backend
	$(BACKEND_BIN) -config config/sentinel.yaml

run-proxy: $(PROXY_BIN) ## Run the proxy
	$(PROXY_BIN) -config config/sentinel.yaml

run: build ## Run both backend and proxy (backend in background)
	@echo "Starting backend..."
	$(BACKEND_BIN) -config config/sentinel.yaml &
	@sleep 1
	@echo "Starting proxy..."
	$(PROXY_BIN) -config config/sentinel.yaml

demo: ## Run the demo script
	./scripts/demo.sh

test: ## Run tests
	$(GO) test -v -race -count=1 ./...

lint: ## Run linter
	golangci-lint run ./...

tidy: ## Tidy go modules
	$(GO) mod tidy

check: build test lint ## Build, test, and lint

help: 
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
