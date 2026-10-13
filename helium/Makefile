# -- Proto Gen --
PROTO_DIR := ./proto
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)
OUTPUT_DIR := ./pb
generate: $(PROTO_FILES)
	@mkdir -p $(OUTPUT_DIR)
	@echo "Generating code for .proto files..."
	@for file in $(PROTO_FILES); do \
		protoc --go_out=$(OUTPUT_DIR) --go-grpc_out=$(OUTPUT_DIR) $$file; \
	done
	@echo "Code generation complete"

# -- OpenAPI Gen --
INPUT_FILE = ./swagger.json
OUTPUT_FILE = ./openapi.json
openapi-gen:
	$(MAKE) -C .. docs-tools
	bun ../docs/generate.ts convert $(INPUT_FILE) $(OUTPUT_FILE)

docs-fmt:
	swag fmt -g src/cmd/server/main.go -d ./src

docs: docs-fmt
	swag init -g src/cmd/server/main.go --ot json -o ./
	make openapi-gen

# -- Migrations --
MIGRATIONS_DIR := src/migration/migrations
DATABASE_URL ?= postgres://snowflake:snowflake@127.0.0.1:5432/helium?sslmode=disable

migrate-up:
	@DATABASE_URL=$(DATABASE_URL) go run src/cmd/migrate/main.go up

migrate-down:
	@DATABASE_URL=$(DATABASE_URL) go run src/cmd/migrate/main.go down

migrate-create:
	@[ -n "$(NAME)" ] || (echo "Usage: make migrate-create NAME=add_foo_column" && exit 1)
	@go run -mod=mod github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 \
		create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)
