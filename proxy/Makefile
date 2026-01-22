APP_NAME := "dnsproxy"

.PHONY: all dep build test coverage coverhtml lint

all: build

# FOLLOWING COMMANDS HAVE TO BE RUN INSIDE DNS PROXY DOCKER CONTAINER

gow: # enable auto app reload on file change
	go install github.com/mitranim/gow@latest
	gow run .

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
	go build -o ${APP_NAME}

test: ## Run all go-based tests
	go test -race -coverprofile=coverage.txt -covermode=atomic -v ./...

mockery: ## Generate mocks
	go install github.com/vektra/mockery/v2@v2.53.5
	mockery --all --case underscore --exclude vendor/ --output ./mocks

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
