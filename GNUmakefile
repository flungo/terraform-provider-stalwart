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

.PHONY: testacc
testacc: ## Run acceptance tests. Boots a throwaway Stalwart container automatically (needs Docker).
	# The test harness starts a disposable Stalwart container, so no instance or
	# credentials need to be supplied. Point the suite at your own server instead
	# by exporting STALWART_ENDPOINT (and STALWART_TOKEN or STALWART_USERNAME /
	# STALWART_PASSWORD). Override the image with STALWART_TEST_IMAGE.
	TF_ACC=1 go test ./internal/provider/... -v $(TESTARGS) -timeout=30m

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
