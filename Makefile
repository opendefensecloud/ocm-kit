# Include ODC common make targets
DEV_KIT_VERSION := v1.0.4
-include common.mk
common.mk:
	curl --fail -sSL https://raw.githubusercontent.com/opendefensecloud/dev-kit/$(DEV_KIT_VERSION)/common.mk -o common.mk.download && \
	mv common.mk.download $@

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format code
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: lint
lint: lint-no-golangci golangci-lint ## Lint code

.PHONY: lint-no-golangci
lint-no-golangci: shellcheck ## Run linters but not golangci-lint to exit early in CI/CD pipeline

.PHONY: test
test: ## Run all tests (except E2E)
	$(GO) test -coverprofile=ocm-kit.coverprofile -v ./...

.PHONY: e2e
e2e: $(OCM) ## Run e2e tests
	OCM=$(OCM) ./test/e2e.sh $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-keep-zot
e2e-keep-zot: # Run e2e tests, but keep zot running
	OCM=$(OCM) ./test/e2e.sh --keep-zot $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-stop-zot
e2e-stop-zot: # Stop and remove zot container
	$(DOCKER) stop zot-registry
	$(DOCKER) rm -f zot-registry
	$(DOCKER) volume rm zot-data
