.PHONY: help


help: ## Displays the help for each command
	@grep -E '^[a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install_test_dependencies: ## Install the test dependencies
	pip install -r requirements.txt

test_ci: ## Run the tests in CI mode
	pytest -s dns_tests/

test_failover: ## Run the destructive Redis failover tests (excluded from test_ci by default addopts)
	pytest -s -m redis_failover dns_tests/infra/

venv:
	source venv/bin/activate

test_local:
	source venv/bin/activate && pytest -s dns_tests/
