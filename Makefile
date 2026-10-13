MVN ?= mvn

.PHONY: docs-tools docs-helium docs-gold docs-carbon docs-oxygen docs-generate openapi docs-check

# --- Checks for required tools ---
check-docker:
	@command -v docker > /dev/null 2>&1 || { echo "Docker is not installed. Please install Docker."; exit 1; }

check-dotnet:
	@command -v dotnet > /dev/null 2>&1 || { echo ".NET is not installed. Please install .NET."; exit 1; }

check-node:
	@command -v node > /dev/null 2>&1 || { echo "Node is not installed. Please install Node."; exit 1; }

check-go:
	@command -v go > /dev/null 2>&1 || { echo "Go is not installed. Please install Go."; exit 1; }

check-swag:
	@command -v swag > /dev/null 2>&1 || { echo "swag is not installed. Please install swag (go install github.com/swaggo/swag/cmd/swag@latest)."; exit 1; }

# --- Database migration targets ---
migrate-oxygen: check-dotnet
	dotnet ef database update --project oxygen/Loan.API

# --- Docker and service start targets ---
start: check-docker check-dotnet
	docker-compose up -d
	make migrate-oxygen

# --- OpenAPI generation targets (individual services) ---
docs/node_modules/.package-lock.json: docs/package.json docs/package-lock.json
	npm ci --prefix docs --no-audit --no-fund

docs-tools: check-node docs/node_modules/.package-lock.json

docs-helium: docs-tools check-swag
	$(MAKE) -C helium docs

docs-gold: docs-tools check-swag
	$(MAKE) -C gold docs

docs-carbon:
	cd carbon && $(MVN) --batch-mode --no-transfer-progress test -Dtest=OpenApiExportTest -Dopenapi.output.path=$(CURDIR)/carbon/openapi.json

docs-oxygen: check-dotnet
	cd oxygen/Oxygen.API && dotnet run --no-launch-profile -- --export-openapi $(CURDIR)/oxygen/openapi.json

# --- OpenAPI generation (all services) ---
docs-generate: docs-helium docs-gold docs-carbon docs-oxygen
	@echo "✓ All service OpenAPI specs generated"

# --- OpenAPI merge related ---
openapi: docs-tools docs-generate
	node docs/generate.mjs merge docs/openapi-merge.json
	@echo "✓ OpenAPI documentation merged at docs/openapi.json"

docs-check: docs-tools
	npm --prefix docs test
	npm --prefix docs run lint
