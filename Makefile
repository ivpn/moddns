CWD=$$(pwd)
.PHONY: help announcements

help: ## Displays the help for each command.
	@grep -E '^[a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

up: ## Starts all of the services.
	docker compose \
		-f compose.yml \
		-f compose.app.yml \
		-f compose.nginx.yml \
		-f compose.db.yml \
		-f compose.redis.yml \
		-f compose.dns.yml \
		-f compose.dnscheck.yml \
		-f compose.knot.yml \
		-f compose.sdns.yml \
		up -d;

down: ## Stops all of the services.
	docker compose -f compose.yml \
	-f compose.app.yml \
	-f compose.nginx.yml \
	-f compose.db.yml \
	-f compose.redis.yml \
	-f compose.dns.yml \
	-f compose.dnscheck.yml \
	-f compose.sdns.yml \
	-f compose.knot.yml \
	down; \
	docker kill -a

up_dns: ## Starts the DNS services (both recursors: sdns + knot).
	docker compose \
	-f compose.yml \
	-f compose.redis.yml \
	-f compose.dns.yml \
	-f compose.sdns.yml \
	-f compose.knot.yml \
	up -d

down_dns: ## Stops the DNS services (both recursors: sdns + knot).
	docker compose \
	-f compose.yml \
	-f compose.redis.yml \
	-f compose.dns.yml \
	-f compose.sdns.yml \
	-f compose.knot.yml \
	down

up_dev: ## Starts the services for development purposes.
	docker compose \
		-f compose.yml \
		-f compose.dev.yml \
		-f compose.redis.yml \
		-f compose.knot.yml \
		-f compose.sdns.yml \
		up -d

down_dev: ## Stops the development services.
	docker compose \
	-f compose.yml \
	-f compose.dev.yml \
	-f compose.redis.yml \
	-f compose.sdns.yml \
	-f compose.knot.yml \
	down --remove-orphans --timeout 10

restart_dev: ## Restarts development services (down + up with proper wait).
	docker compose \
	-f compose.dev.yml \
	-f compose.yml \
	-f compose.sdns.yml \
	-f compose.knot.yml \
	down --remove-orphans --timeout 10
	@echo "Waiting for network resources to be released..."
	@sleep 2
	docker compose \
		-f compose.yml \
		-f compose.dev.yml \
		-f compose.knot.yml \
		-f compose.sdns.yml \
		up -d

announcements: ## Serves the Announcements dev fixture on the dns network as http://announcements-dev/ (set ANNOUNCEMENTS_URL in api/.env + recreate dnsapi; see README-dev.md).
	@NET=$$(docker inspect -f '{{range $$k,$$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' dnsapi 2>/dev/null); \
		if [ -z "$$NET" ]; then echo "dnsapi container not found — start the stack first (make up / make up_dev)."; exit 1; fi; \
		docker rm -f announcements-dev >/dev/null 2>&1 || true; \
		echo "Serving bootstrap/announcements/ on network $$NET as http://announcements-dev/announcements.md  (Ctrl-C to stop)"; \
		echo "api/.env must have ANNOUNCEMENTS_URL=http://announcements-dev/announcements.md and ANNOUNCEMENTS_RELOAD=10s; recreate dnsapi once after setting them (env is read at container start)."; \
		docker run --rm --name announcements-dev --network "$$NET" -v "$$(pwd)/bootstrap/announcements:/usr/share/nginx/html:ro" nginx:alpine

IMAGE?=dnsapi
build_api_image: ## Builds the DNS REST API image.
	docker build -t ${IMAGE} -f api/Dockerfile .

IMAGE?=dnsproxy
build_proxy_image: ## Builds the DNS Proxy image.
	docker build -t ${IMAGE} -f proxy/Dockerfile .

IMAGE?=dnscheck
build_dnscheck_image: ## Builds the DNS check image.
	docker build -t ${IMAGE} -f dnscheck/Dockerfile .

IMAGE?=dnsblocklists
build_blocklists_image: ## Builds the DNS Blocklists image.
	docker build -t ${IMAGE} -f blocklists/Dockerfile .

IMAGE?=dnswebapp
ENVIRONMENT?=staging
build_frontend_image: ## Builds the DNS Webapp image.
	docker build -t ${IMAGE} app/ --build-arg ENVIRONMENT=${ENVIRONMENT}

dev_api: ## Starts the development api service.
	docker exec -it dnsapi make gow

dev_blocklists: ## Starts the development blocklists service.
	docker exec -it dnsblocklists make gow

dev_proxy: ## Starts the development proxy service.
	docker exec -it dnsproxy make gow

dev_check: ## Starts the development dnscheck service.
	docker exec -it dnscheck make gow

gen_python_client: ## Generates the python client from swagger spec (renamed to moddns_client, package moddns).
	rm -r tests/moddns_client/ || true
	docker run -v ${CWD}:/app -w /app/api/docs --user $$(id -u):$$(id -g) --rm -i openapitools/openapi-generator-cli generate \
		--package-name moddns \
		-i swagger.yaml \
		-g python \
		-o /app/tests/moddns_client \
		--skip-validate-spec \
		--global-property models,apis,supportingFiles,apiTests=false,modelTests=false,apiDocs=false,modelDocs=false

gen_ts_client: ## Generates the typescript client from swagger spec.
	rm -rf app/src/api/client/ || true
	docker run -v ${CWD}:/app -w /app/api/docs --user $$(id -u):$$(id -g) --rm openapitools/openapi-generator-cli generate --package-name idns -i swagger.yaml -g typescript-axios -o /app/app/src/api/client --skip-validate-spec

build_tests_image: ## Builds the smoke / backend E2E tests image.
	docker build -f tests/Dockerfile -t dns_tests:latest .

dev_tests: ## Starts the development tests docker container.
	docker run --network host -it --rm -v ${CWD}:/app -w /app dns_tests:latest

### BUILD DEV IMAGES
image?=api
build_image_dev:
	@if [ ${image} = "api" ]; then \
		echo "Building DNS API dev image...\n"; \
		docker build -t dnsapidev -f api/Dockerfile.dev . ;\
	fi
	@if [ ${image} = "proxy" ]; then \
		echo "Building DNS Proxy dev image...\n"; \
		docker build -t dnsproxydev -f proxy/Dockerfile.dev . ;\
	fi
		@if [ ${image} = "dnscheck" ]; then \
		echo "Building DNS check dev image...\n"; \
		docker build -t dnscheckdev -f dnscheck/Dockerfile.dev . ;\
	fi
		@if [ ${image} = "blocklists" ]; then \
		echo "Building DNS Blocklists dev image...\n"; \
		docker build -t dnsblocklistsdev -f blocklists/Dockerfile.dev . ;\
	fi
