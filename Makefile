GO_PACKAGES ?= $(shell go list ./... | grep -v /vendor/)

TARGETOS ?= $(shell go env GOOS)
TARGETARCH ?= $(shell go env GOARCH)

DIST_DIR ?= dist

VERSION ?= next
VERSION_NUMBER ?= 0.0.0
CI_COMMIT_SHA ?= $(shell git rev-parse HEAD)

# it's a tagged release
ifneq ($(CI_COMMIT_TAG),)
	VERSION := $(CI_COMMIT_TAG:v%=%)
	VERSION_NUMBER := ${CI_COMMIT_TAG:v%=%}
else
	# append commit-sha to next version
	ifeq ($(VERSION),next)
		VERSION := $(shell echo "next-$(shell echo ${CI_COMMIT_SHA} | cut -c -10)")
	endif
	# append commit-sha to release branch version
	ifeq ($(shell echo ${CI_COMMIT_BRANCH} | cut -c -9),release/v)
		VERSION := $(shell echo "$(shell echo ${CI_COMMIT_BRANCH} | cut -c 10-)-$(shell echo ${CI_COMMIT_SHA} | cut -c -10)")
	endif
endif

TAGS ?=
LDFLAGS := -X go.codycody31.dev/squad-aegis/version.Version=${VERSION}
STATIC_BUILD ?= false
ifeq ($(STATIC_BUILD),true)
	LDFLAGS := -s -w -extldflags "-static" $(LDFLAGS)
endif
CGO_ENABLED ?= 1 # only used to compile server

HAS_GO = $(shell hash go > /dev/null 2>&1 && echo "GO" || echo "NOGO" )
ifeq ($(HAS_GO),GO)
  # renovate: datasource=docker depName=docker.io/techknowlogick/xgo
	XGO_VERSION ?= go-1.23.x
	CGO_CFLAGS ?= $(shell go env CGO_CFLAGS)
endif
CGO_CFLAGS ?=

# Proceed with normal make

##@ General

.PHONY: all
all: help

.PHONY: version
version: ## Print the current version
	@echo ${VERSION}

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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: vendor
vendor: ## Update the vendor directory
	go mod tidy
	go mod vendor

format: install-tools ## Format source code
	@gofumpt -extra -w .

.PHONY: generate
generate: install-tools ## Run all code generations
	CGO_ENABLED=0 go generate ./...

install-tools: ## Install development tools
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest ; \
	fi

##@ Test

.PHONY: lint
lint: install-tools ## Lint code
	@echo "Running golangci-lint"
	golangci-lint run

.PHONY: test ## Run tests
test:
	go test -race -cover -coverprofile server-coverage.out -timeout 60s -tags 'test' go.codycody31.dev/squad-aegis/...

##@ Build

build-web: ## Build Web UI
	(cd web/; corepack enable; pnpm install --frozen-lockfile; pnpm run build)

build: build-web generate-swagger ## Build server
	CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags '$(TAGS)' -ldflags '${LDFLAGS}' -o ${DIST_DIR}/squad-aegis go.codycody31.dev/squad-aegis/cmd

build-tarball: ## Build tar archive
	mkdir -p ${DIST_DIR} && tar chzvf ${DIST_DIR}/squad-aegis-src.tar.gz \
	  --exclude="*.exe" \
	  --exclude="./.pnpm-store" \
	  --exclude="node_modules" \
	  --exclude="./dist" \
	  --exclude="./data" \
	  --exclude="./build" \
	  --exclude="./.git" \
	  .

.PHONY: build
build: build ## Build all binaries

release: ## Create binaries for release
	GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) CGO_ENABLED=${CGO_ENABLED} go build  -ldflags '${LDFLAGS}' -tags '$(TAGS)' -o ${DIST_DIR}/$(TARGETOS)_$(TARGETARCH)/squad-aegis go.codycody31.dev/squad-aegis/cmd
	tar -czf ${DIST_DIR}/squad-aegis_$(TARGETOS)_$(TARGETARCH).tar.gz -C ${DIST_DIR}/$(TARGETOS)_$(TARGETARCH) squad-aegis

release-checksums: ## Create checksums for all release files
	# generate shas for tar files
	(cd ${DIST_DIR}/; sha256sum *.* > checksums.txt)