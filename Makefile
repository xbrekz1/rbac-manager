# Makefile for rbac-manager

# Variables
BINARY_NAME=manager
IMAGE_NAME=ghcr.io/xbrekz1/rbac-manager
VERSION?=latest
GO=go
KUBECTL=kubectl
HELM=helm

# Go parameters
GOCMD=$(GO)
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Directories
BUILD_DIR=bin
COVER_DIR=coverage

.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean fmt vet lint test build ## Run all checks and build

.PHONY: build
build: ## Build the binary
	@echo "Building..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/main.go

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=$(COVER_DIR)/coverage.txt -covermode=atomic ./...

.PHONY: test-unit
test-unit: ## Run only unit tests (without integration tests)
	@echo "Running unit tests..."
	$(GOTEST) -v -race -short ./...

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	@mkdir -p $(COVER_DIR)
	$(GOCMD) tool cover -html=$(COVER_DIR)/coverage.txt -o $(COVER_DIR)/coverage.html
	@echo "Coverage report: $(COVER_DIR)/coverage.html"

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout=5m

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(COVER_DIR)

.PHONY: deps
deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

.PHONY: tidy
tidy: ## Tidy Go modules
	@echo "Tidying modules..."
	$(GOMOD) tidy

.PHONY: verify
verify: fmt vet lint test ## Run all verification checks

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME):$(VERSION) .

.PHONY: docker-push
docker-push: ## Push Docker image
	@echo "Pushing Docker image..."
	docker push $(IMAGE_NAME):$(VERSION)

.PHONY: docker-buildx
docker-buildx: ## Build multi-arch Docker image
	@echo "Building multi-arch Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME):$(VERSION) --push .

.PHONY: install
install: ## Install the operator using Helm
	@echo "Installing rbac-manager..."
	$(HELM) install rbac-manager . --namespace rbac-manager --create-namespace --wait

.PHONY: uninstall
uninstall: ## Uninstall the operator
	@echo "Uninstalling rbac-manager..."
	$(HELM) uninstall rbac-manager --namespace rbac-manager

.PHONY: upgrade
upgrade: ## Upgrade the operator
	@echo "Upgrading rbac-manager..."
	$(HELM) upgrade rbac-manager . --namespace rbac-manager --wait

.PHONY: helm-lint
helm-lint: ## Lint Helm chart
	@echo "Linting Helm chart..."
	$(HELM) lint . --strict

.PHONY: generate
generate: ## Generate deepcopy code
	@echo "Generating code..."
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: ## Generate CRD manifests
	@echo "Generating manifests..."
	controller-gen crd:crdVersions=v1 rbac:roleName=rbac-manager webhook paths="./..." output:crd:artifacts:config=templates

.PHONY: run
run: fmt vet ## Run the operator locally
	@echo "Running operator locally..."
	$(GOCMD) run ./cmd/main.go

.PHONY: dev
dev: ## Run in development mode with hot reload (requires air)
	@which air > /dev/null || (echo "air not found. Install with: go install github.com/cosmtrek/air@latest" && exit 1)
	air

.PHONY: example-create
example-create: ## Create example AccessGrant
	@echo "Creating example AccessGrant..."
	$(KUBECTL) apply -f examples.yaml

.PHONY: example-delete
example-delete: ## Delete example AccessGrant
	@echo "Deleting example AccessGrant..."
	$(KUBECTL) delete -f examples.yaml

.PHONY: logs
logs: ## Show operator logs
	@echo "Showing logs..."
	$(KUBECTL) logs -n rbac-manager -l app.kubernetes.io/name=rbac-manager -f

.PHONY: status
status: ## Show AccessGrants status
	@echo "AccessGrants:"
	$(KUBECTL) get accessgrants -A

.PHONY: setup-envtest
setup-envtest: ## Setup envtest binaries
	@echo "Setting up envtest..."
	@which setup-envtest > /dev/null || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	setup-envtest use

.PHONY: build-kubeconfigctl
build-kubeconfigctl: ## Build kubeconfigctl CLI tool
	@echo "Building kubeconfigctl..."
	cd cmd/kubeconfigctl && $(GOBUILD) -o kubeconfigctl -ldflags="-X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" .
	@echo "✓ Built: cmd/kubeconfigctl/kubeconfigctl"
	@echo ""
	@echo "To install globally:"
	@echo "  sudo mv cmd/kubeconfigctl/kubeconfigctl /usr/local/bin/"

.PHONY: install-kubeconfigctl
install-kubeconfigctl: build-kubeconfigctl ## Build and install kubeconfigctl
	@echo "Installing kubeconfigctl to /usr/local/bin..."
	sudo mv cmd/kubeconfigctl/kubeconfigctl /usr/local/bin/
	@echo "✓ Installed!"
	@echo ""
	kubeconfigctl version

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing controller-gen..."
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@echo "Installing setup-envtest..."
	@go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	@echo "Installing ginkgo..."
	@go install github.com/onsi/ginkgo/v2/ginkgo@latest
	@echo "Installing spf13/cobra..."
	@go install github.com/spf13/cobra-cli@latest
	@echo "All tools installed!"

.PHONY: pre-commit
pre-commit: fmt vet lint test ## Run pre-commit checks
	@echo "✓ Pre-commit checks passed!"

.DEFAULT_GOAL := help
