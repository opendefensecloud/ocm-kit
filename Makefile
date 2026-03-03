
# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Set MAKEFLAGS to suppress entering/leaving directory messages
MAKEFLAGS += --no-print-directory

BUILD_PATH ?= $(shell pwd)
HACK_DIR ?= $(shell cd hack 2>/dev/null && pwd)
LOCALBIN ?= $(BUILD_PATH)/bin

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

GO ?= go
DOCKER ?= docker
SHELLCHECK ?= shellcheck
OSV_SCANNER ?= osv-scanner
OCM ?= $(LOCALBIN)/ocm

OCM_VERSION ?= 0.35.0

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean all temporary resources
	rm -rf $(LOCALBIN)

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...


.PHONY: scan
scan:
	$(OSV_SCANNER) scan --config ./.osv-scanner.toml -r .

.PHONY: lint
lint: ## Lint code
	$(SHELLCHECK) test/e2e.sh

.PHONY: mod
mod: ## Do go mod tidy, download, verify
	@$(GO) mod tidy
	@$(GO) mod download
	@$(GO) mod verify

.PHONY: test
test: ## Run all tests (except E2E)
	$(GO) test -coverprofile=ocm-kit.coverprofile -v ./...

.PHONY: e2e
e2e: ## Run e2e tests
	OCM=$(OCM) ./test/e2e.sh $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-keep-zot
e2e-keep-zot: # Run e2e tests, but keep zot running
	OCM=$(OCM) ./test/e2e.sh --keep-zot $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-stop-zot
e2e-stop-zot: # Stop and remove zot container
	$(DOCKER) stop zot-registry
	$(DOCKER) rm -f zot-registry
	$(DOCKER) volume rm zot-data

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: ocm
ocm: $(OCM) ## Download ocm locally if necessary.
$(OCM): $(LOCALBIN)
	test -s $(LOCALBIN)/ocm || (curl -L -o $(LOCALBIN)/ocm.tar.gz "https://github.com/open-component-model/ocm/releases/download/v$(OCM_VERSION)/ocm-$(OCM_VERSION)-$(OS)-$(ARCH).tar.gz"; tar -xvf $(LOCALBIN)/ocm.tar.gz -C $(LOCALBIN); chmod +x $(LOCALBIN)/ocm; rm $(LOCALBIN)/ocm.tar.gz)

