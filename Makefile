# Makefile for MCP Immich Server

# Variables
BINARY_NAME=mcp-immich
DOCKER_IMAGE=mcp-immich
VERSION?=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-w -s -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Directories
PKG_DIR=./pkg/...
CMD_DIR=./cmd/mcp-immich
TEST_DIR=./test/...
CONFIG_FILE?=config.yaml

.PHONY: all build clean test coverage fmt vet lint docker help

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## all: Build and test
all: test build

## build: Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)

## build-linux: Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 $(CMD_DIR)

## build-darwin: Build for macOS
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

## build-windows: Build for Windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

## clean: Remove build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html

## test: Run tests
test:
	$(GOTEST) -v -race -timeout 30s $(PKG_DIR)

## test-short: Run short tests
test-short:
	$(GOTEST) -v -short $(PKG_DIR)

## test-coverage: Run tests with coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out -covermode=atomic $(PKG_DIR)
	@echo ""
	@echo "Coverage Report:"
	@$(GOCMD) tool cover -func=coverage.out

## test-coverage-html: Generate HTML coverage report
test-coverage-html: test-coverage
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## check-coverage: Check if coverage meets minimum threshold (80%)
check-coverage: test-coverage
	@echo "Checking coverage threshold (minimum 80%)..."
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $$COVERAGE%"; \
	if [ $$(echo "$$COVERAGE < 80" | bc) -eq 1 ]; then \
		echo "❌ Coverage is below 80% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage meets 80% threshold"; \
	fi

## test-smoke: Run smoke tests (requires test environment)
test-smoke:
	$(GOTEST) -v -tags=smoke $(TEST_DIR)

## test-integration: Run integration tests
test-integration:
	$(GOTEST) -v -tags=integration $(TEST_DIR)/integration/...

## benchmark: Run benchmarks
benchmark:
	$(GOTEST) -bench=. -benchmem $(PKG_DIR)

## fmt: Format code
fmt:
	$(GOFMT) -w -s .
	$(GOCMD) fmt $(PKG_DIR)
	$(GOCMD) fmt $(CMD_DIR)

## vet: Run go vet
vet:
	$(GOCMD) vet $(PKG_DIR)
	$(GOCMD) vet $(CMD_DIR)

## lint: Run golangci-lint
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

## mod-download: Download dependencies
mod-download:
	$(GOMOD) download

## mod-tidy: Tidy dependencies
mod-tidy:
	$(GOMOD) tidy

## mod-verify: Verify dependencies
mod-verify:
	$(GOMOD) verify

## docker-build: Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

## docker-push: Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

## docker-run: Run Docker container
docker-run:
	docker run -it --rm \
		-p 8080:8080 \
		-e MCP_IMMICH_URL=$${MCP_IMMICH_URL} \
		-e MCP_IMMICH_API_KEY=$${MCP_IMMICH_API_KEY} \
		$(DOCKER_IMAGE):latest

## docker-compose-up: Start services with docker-compose
docker-compose-up:
	docker-compose up -d

## docker-compose-down: Stop services with docker-compose
docker-compose-down:
	docker-compose down

## run: Run the server locally
run:
	$(GOCMD) run $(CMD_DIR) -config $(CONFIG_FILE)

## run-stdio: Run the server with stdio transport
run-stdio:
	$(GOCMD) run $(CMD_DIR) -config $(CONFIG_FILE) -stdio

## install: Install the binary
install: build
	cp $(BINARY_NAME) $(GOPATH)/bin/

## generate: Generate mocks and other code
generate:
	@which mockgen > /dev/null || go install github.com/golang/mock/mockgen@latest
	go generate ./...

## security-scan: Run security scan
security-scan:
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

## deps-check: Check for dependency updates
deps-check:
	@which go-mod-outdated > /dev/null || go install github.com/psampaz/go-mod-outdated@latest
	go list -u -m -json all | go-mod-outdated -direct

## ci: Run CI pipeline locally
ci: mod-tidy fmt vet lint test check-coverage build

## release: Create a new release
release: ci
	@echo "Creating release $(VERSION)..."
	@echo "Building binaries..."
	@make build-linux
	@make build-darwin
	@make build-windows
	@echo "Release $(VERSION) ready!"

.DEFAULT_GOAL := help
