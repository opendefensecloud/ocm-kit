
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
clean: zot-stop ocm-clean-ctf ## Clean all temporary resources
	rm -rf $(LOCALBIN)

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...

.PHONY: mod
mod: ## Do go mod tidy, download, verify
	@$(GO) mod tidy
	@$(GO) mod download
	@$(GO) mod verify

zot-start:
	$(DOCKER) run -d -p 5000:5000 \
	  --name zot-registry \
	  -v $(BUILD_PATH)/test/fixtures/zot-config.json:/etc/zot/config.json:ro \
	  -v zot-data:/var/lib/registry \
	  ghcr.io/project-zot/zot:v2.1.10

zot-stop:
	$(DOCKER) stop zot-registry
	$(DOCKER) rm -f zot-registry
	$(DOCKER) volume rm zot-data

ocm-ctf: ocm
	cd test/fixtures/arc && $(OCM) add componentversion --version 0.1.0 --create --file ./ctf component-constructor.yaml

ocm-clean-ctf:
	rm -rf test/fixtures/arc/ctf/

ocm-transfer: ocm ocm-ctf
	$(OCM) transfer ctf --copy-resources ./test/fixtures/arc/ctf http://localhost:5000/my-components

run:
	$(GO) run cmd/ocm-kit/main.go

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: ocm
ocm: $(OCM) ## Download ocm locally if necessary.
$(OCM): $(LOCALBIN)
	test -s $(LOCALBIN)/ocm || (curl -L -o $(LOCALBIN)/ocm.tar.gz "https://github.com/open-component-model/ocm/releases/download/v$(OCM_VERSION)/ocm-$(OCM_VERSION)-$(OS)-$(ARCH).tar.gz"; tar -xvf $(LOCALBIN)/ocm.tar.gz -C $(LOCALBIN); chmod +x $(LOCALBIN)/ocm; rm $(LOCALBIN)/ocm.tar.gz)

