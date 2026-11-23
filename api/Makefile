CWD=$$(pwd)
PKG := "/app"
PKG_LIST := $(shell go list ${PKG}/...)

.PHONY: all dep build test coverage coverhtml lint

all: build

gow: # enable auto app reload on file change
	go install github.com/mitranim/gow@latest
	gow run .

run:
	docker run --network host --rm -it --env-file .env dnsapi

install_swag:
	go install github.com/swaggo/swag/cmd/swag@latest

swag:
	swag init --parseDependency  --parseInternal --parseDepth 10

mockery: ## Generate mocks (uses .mockery.yaml config)
	go install github.com/vektra/mockery/v3@v3.6.1
	mockery

# FOLLOWING COMMANDS HAVE TO BE RUN INSIDE DNS API DOCKER CONTAINER

lint: ## Lint the files
	golangci-lint -c ../.github/golangci.yml run --fix

coverage: ## Generate global code coverage report ()
	@go test -covermode=atomic -coverprofile coverage.out -v ./...

coverhtml: coverage ## Generate global code coverage report in HTML
	@go tool cover -html=coverage.out -o coverage.html


dep: ## Get the test/lint dependencies
	@go get -u golang.org/x/lint/golint
	@go get github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.40.1
	@go get golang.org/x/tools/cmd/goimports

build: ## Build DNS Proxy application
	go build -o dnsapi

test: ## Run all go-based tests
	go test -p 1 -coverprofile=coverage.txt -covermode=atomic -v ./...

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
