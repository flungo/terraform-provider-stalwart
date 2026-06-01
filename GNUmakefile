# Copyright (c) Fabrizio Lungo
# SPDX-License-Identifier: MPL-2.0

HOSTNAME    := registry.terraform.io
NAMESPACE   := flungo
NAME        := stalwart
BINARY      := terraform-provider-$(NAME)
VERSION     := 0.1.0
OS_ARCH     := $(shell go env GOOS)_$(shell go env GOARCH)

# Local plugin mirror used by `make install` so that the freshly-built provider
# can be referenced from a local Terraform configuration.
INSTALL_DIR := ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

default: build

.PHONY: build
build: ## Compile the provider binary.
	go build -o $(BINARY)

.PHONY: install
install: build ## Build and install the provider into the local plugin mirror.
	mkdir -p $(INSTALL_DIR)
	mv $(BINARY) $(INSTALL_DIR)/$(BINARY)_v$(VERSION)

.PHONY: test
test: ## Run unit tests.
	go test ./... $(TESTARGS) -timeout=120s

# COVERPKG is the set of packages whose statements count toward coverage: the
# shipped provider and client code. The internal/acctest harness is test
# infrastructure (it boots containers) and is deliberately excluded from the
# denominator.
COVERPKG := github.com/flungo/terraform-provider-stalwart/internal/provider,github.com/flungo/terraform-provider-stalwart/internal/client
# COVERAGE_MIN is the minimum total statement coverage enforced by `make
# cover-check` and the CI acceptance job. It is a floor to be ratcheted upward
# toward the actual figure (reported in the CI job summary) over time.
COVERAGE_MIN ?= 75

.PHONY: testacc
testacc: ## Run acceptance tests with coverage. Boots a throwaway Stalwart container automatically (needs Docker).
	# The test harness starts a disposable Stalwart container, so no instance or
	# credentials need to be supplied. Point the suite at your own server instead
	# by exporting STALWART_ENDPOINT (and STALWART_TOKEN or STALWART_USERNAME /
	# STALWART_PASSWORD). Override the image with STALWART_TEST_IMAGE.
	TF_ACC=1 go test ./internal/provider/... -v $(TESTARGS) -timeout=30m \
		-coverpkg=$(COVERPKG) -coverprofile=coverage.out

.PHONY: cover-check
cover-check: ## Fail if total coverage (from coverage.out) is below COVERAGE_MIN.
	./scripts/coverage-check.sh coverage.out $(COVERAGE_MIN)

.PHONY: cover-html
cover-html: ## Render coverage.out as an HTML report at coverage.html.
	go tool cover -html=coverage.out -o coverage.html

.PHONY: generate
generate: ## Generate documentation and other code-generated artifacts.
	cd tools && go generate ./...

.PHONY: lint
lint: ## Run gofmt, go vet, and golangci-lint.
	gofmt -l -s .
	go vet ./...
	golangci-lint run

.PHONY: fmt
fmt: ## Format Go source.
	gofmt -w -s .

.PHONY: help
help: ## Display this help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'
