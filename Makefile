CWD=$$(pwd)
.PHONY: help

help: ## Displays the help for each command.
	@grep -E '^[a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

RECURSOR?=unbound # unbound is default recursor
up: ## Starts all of the services.
	docker compose \
		-f compose.yml \
		-f compose.app.yml \
		-f compose.nginx.yml \
		-f compose.db.yml \
		-f compose.redis.yml \
		-f compose.dns.yml \
		-f compose.dnscheck.yml \
		-f compose.unbound.yml \
		-f compose.sdns.yml \
		up -d;

	# @if [ ${RECURSOR} = "unbound" ]; then \
	# 	echo "Using unbound recursor...\n";\
    #   	docker compose \
	# 	-f compose.yml \
	# 	-f compose.app.yml \
	# 	-f compose.nginx.yml \
	# 	-f compose.db.yml \
	# 	-f compose.redis.yml \
	# 	-f compose.dns.yml \
	# 	-f compose.dnscheck.yml \
	# 	-f compose.unbound.yml \
	# 	up -d;\
    # else \
	# 	echo "Using SDNS recursor...\n";\
	#   	docker compose -f compose.yml \
	# 	-f compose.app.yml \
	# 	-f compose.nginx.yml \
	# 	-f compose.redis.yml \
	# 	-f compose.dns.yml \
	# 	-f compose.dnscheck.yml \
	# 	-f compose.sdns.yml \
	# 	up -d; \
	# fi

down: ## Stops all of the services.
	docker compose -f compose.yml \
	-f compose.app.yml \
	-f compose.nginx.yml \
	-f compose.db.yml \
	-f compose.redis.yml \
	-f compose.dns.yml \
	-f compose.dnscheck.yml \
	-f compose.sdns.yml \
	-f compose.unbound.yml \
	down; \
	docker kill -a

up_dns: ## Starts the DNS services.
	@if [ ${RECURSOR} = "sdns" ]; then \
		echo "Using SDNS recursor...\n";\
	  	docker compose \
		-f compose.yml \
		-f compose.dns.yml \
		-f compose.sdns.yml up -d;\
	else \
		echo "Using Unbound recursor...\n";\
	  	docker compose \
		-f compose.yml \
		-f compose.dns.yml \
		-f compose.unbound.yml up -d;\
	fi

down_dns: ## Stops the DNS services.
	@if [ ${RECURSOR} = "sdns" ]; then \
		echo "Starting DNS services with SDNS recursor...\n";\
	  	docker compose \
		-f compose.yml \
		-f compose.dns.yml \
		-f compose.sdns.yml \
		down ;\
	else \
		echo "Starting DNS services with Unbound recursor...\n";\
	  	docker compose \
		-f compose.yml \
		-f compose.dns.yml \
		-f compose.unbound.yml \
		down ;\
	fi

# RECURSOR?=unbound # unbound is default recursor
up_dev: ## Starts the services for development purposes.
	docker compose \
		-f compose.yml \
		-f compose.dev.yml \
		-f compose.redis.yml \
		-f compose.unbound.yml \
		-f compose.sdns.yml \
		up -d

down_dev: ## Stops the development services.
	docker compose \
	-f compose.yml \
	-f compose.dev.yml \
	-f compose.redis.yml \
	-f compose.sdns.yml \
	-f compose.unbound.yml \
	down

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
	sudo rm -r tests/moddns_client/ || true
	docker run -v ${CWD}:/app -w /app/api/docs --rm -it openapitools/openapi-generator-cli generate --package-name moddns -i swagger.yaml -g python -o /app/tests/moddns_client --skip-validate-spec
	sudo chmod -R 777 tests/moddns_client/

gen_ts_client: ## Generates the typescript client from swagger spec.
	sudo rm -r app/src/api/client/ || true
	docker run -v ${CWD}:/app -w /app/api/docs --rm -it openapitools/openapi-generator-cli generate --package-name idns -i swagger.yaml -g typescript-axios -o /app/app/src/api/client --skip-validate-spec
	sudo chmod -R 777 app/src/api/client/

build_tests_image: ## Builds the smoke / integration tests image.
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
