ifndef DOCKERIMAGE
DOCKERIMAGE := syncano/codebox
endif

CURRENTPACKAGE := github.com/Syncano/codebox
EXECNAME := codebox
WRAPPERNAME := codewrapper
WRAPPERPATH = $(shell pwd)/build/codewrapper

PATH := $(PATH):$(GOPATH)/bin
GOFILES = $(shell find . -mindepth 2 -type f -name '*.go' ! -path "./.*" ! -path "./assets/*" ! -path "./dev/*" ! -path "./vendor/*" ! -path "*/mocks/*" ! -path "*/proto/*")
GOTESTPACKAGES = $(shell find . -mindepth 2 -type f -name '*.go' ! -path "./.*" ! -path "./cmd/*" ! -path "./codewrapper/*" ! -path "./assets/*" ! -path "./internal/*" ! -path "./dev/*" ! -path "*/mocks/*" ! -path "*/proto/*" | xargs -n1 dirname | sort | uniq)
BUILDTIME = $(shell date +%Y-%m-%dT%H:%M)
GITSHA = $(shell git rev-parse --short HEAD)

LDFLAGS = -s -w \
	-X github.com/Syncano/codebox/pkg/version.GitSHA=$(GITSHA) \
	-X github.com/Syncano/codebox/pkg/version.buildtimeStr=$(BUILDTIME)


.PHONY: help deps download-images clean lint fmt test stest itest atest cov goconvey lint-in-docker test-in-docker generate-assets generate build build-wrapper build-in-docker docker deploy-staging deploy-production encrypt decrypt start
.DEFAULT_GOAL := help
$(VERBOSE).SILENT:

help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

require-%:
	if ! hash ${*} 2>/dev/null; then \
		echo "! ${*} not installed"; \
		exit 1; \
	fi

download-images: require-docker ## Download wrapper docker images
	docker pull node:12-stretch
	docker pull node:8-stretch

clean: ## Cleanup repository
	go clean ./...
	rm -f build/$(EXECNAME) build/${WRAPPERNAME}
	find deploy -name "*.unenc" -delete
	git clean -f

lint: ## Run lint checks
	echo "=== lint ==="
	if ! hash golangci-lint 2>/dev/null; then \
		echo "Installing golangci-lint"; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $$(go env GOPATH)/bin v1.24.0; \
	fi
	golangci-lint run $(ARGS)

fmt: ## Format code through goimports
	gofmt -s -w $(GOFILES)
	go run golang.org/x/tools/cmd/goimports -local $(CURRENTPACKAGE) -w $(GOFILES)

test: ## Run unit with race check and create coverage profile
	echo "=== unit test ==="
	echo "mode: atomic" > coverage-all.out
	$(foreach pkg,$(GOTESTPACKAGES),\
		go test -timeout 5s -short -race -coverprofile=coverage.out -covermode=atomic $(ARGS) $(pkg) || exit;\
		tail -n +2 coverage.out >> coverage-all.out 2>/dev/null;)

stest: ## Run only short tests (unit tests) without race check
	echo "=== short test ==="
	go test -timeout 5s -short $(ARGS) $(GOTESTPACKAGES)

itest: ## Run only integration tests (with race checks)
	echo "=== integration test ==="
	WRAPPERPATH=$(WRAPPERPATH) \
		go test -timeout 180s -race -run Integration $(ARGS) $(shell echo $(GOFILES) | xargs -n1 echo | grep "_integration_test.go" | xargs -n1 dirname | sort | uniq)

atest: ## Run only acceptance tests
	echo "=== acceptance test ==="
	WRAPPERPATH=$(WRAPPERPATH) \
		go test -timeout 120s -run Acceptance $(ARGS) $(shell echo $(GOFILES) | xargs -n1 echo | grep "_acceptance_test.go" | xargs -n1 dirname | sort | uniq)

cov: ## Show per function coverage generated by test
	echo "=== coverage ==="
	go tool cover -func=coverage-all.out

goconvey: ## Run goconvey test server
	go run github.com/smartystreets/goconvey -excludedDirs "dev,internal,mocks,proto,assets,deploy,build" -timeout 5s -depth 2

lint-in-docker: require-docker-compose ## Run lint in docker environment
	docker-compose run --no-deps --rm app make lint

test-in-docker: require-docker-compose ## Run full test suite in docker environment
	docker-compose run --rm app make build build-wrapper test itest atest

generate-assets: ## Generate assets with go-bindata
	go run github.com/kevinburke/go-bindata/go-bindata -nocompress -nometadata -nomemcopy -prefix assets -o assets/assets.go -pkg assets -ignore assets\.go assets/*

generate: generate-assets ## Run go generate
	go generate ./...

build: ## Build
	go build -ldflags "$(LDFLAGS)" -o ./build/$(EXECNAME)

build-wrapper: ## Build static version of wrapper
	cd codewrapper; GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o ../build/$(WRAPPERNAME)

build-in-docker: require-docker-compose ## Build in docker environment
	docker-compose run --no-deps --rm -e CGO_ENABLED=0 app make build build-wrapper

docker: require-docker ## Builds docker image for application (requires static version to be built first)
	docker build -t $(DOCKERIMAGE) build

deploy-staging: ## Deploy application to staging
	echo "=== deploying staging ==="
	kubectl config use-context k8s.syncano.rocks
	./deploy.sh staging $(GITSHA) $(ARGS)

deploy-production: ## Deploy application to production
	echo "=== deploying us1 ==="
	kubectl config use-context k8s.syncano.io
	./deploy.sh us1 $(GITSHA) $(ARGS)

	echo "=== deploying eu1 ==="
	kubectl config use-context gke_pioner-syncano-prod-9cfb_europe-west1_syncano-eu1
	./deploy.sh eu1 $(GITSHA) --skip-push

encrypt: ## Encrypt unencrypted files (for secrets).
	find deploy -name "*.unenc" -exec sh -c 'gpg --batch --yes --passphrase "$(CODEBOX_VAULT_PASS)" --symmetric --cipher-algo AES256 -o "$${1%.unenc}.gpg" "$$1"' _ {} \;

decrypt: ## Decrypt files.
	find deploy -name "*.gpg" -exec sh -c 'gpg --batch --yes --passphrase "$(CODEBOX_VAULT_PASS)" --decrypt -o "$${1%.gpg}.unenc" "$$1"' _ {} \;

start: require-docker-compose ## Run docker-compose of an app.
	docker-compose -f docker-compose.yml -f build/docker-compose.yml up
