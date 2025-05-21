MAIN=cmd/main.go
GO=go

include .env
export

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make db-init               # Initialize database (create, migrate, seed)"
	@echo "  make migrate-up            # Run all pending migrations"
	@echo "  make migrate-down          # Revert last migration"
	@echo "  make migrate-steps n=N     # Step N migrations up/down (N positive/negative)"
	@echo "  make migrate-create name=NAME  # Create new migration (use 'name' var)"
	@echo "  make migrate-version       # Show current migration version"
	@echo "  make db-seed               # Seed database (env: dev/test/prod)"
	@echo "  make db-seed-custom file=FILE  # Seed using custom SQL file"
	@echo "  make tidy                  # Run go mod tidy"
	@echo "  make build                 # Build binary"
	@echo "  make clean                 # Remove built binary"

.PHONY: db-init
db-init:
	$(GO) run $(MAIN) -cmd=init -env=$(POSTGRES_ENV)

.PHONY: migrate-up
migrate-up:
	$(GO) run $(MAIN) -cmd=up

.PHONY: migrate-down
migrate-down:
	$(GO) run $(MAIN) -cmd=down

.PHONY: migrate-steps
migrate-steps:
	@if [ -z "$(n)" ]; then \
	  echo "Usage: make migrate-steps n=N (N is step count)"; \
	else \
	  $(GO) run $(MAIN) -cmd=steps -steps=$(n); \
	fi

.PHONY: migrate-force
migrate-force:
	@if [ -z "$(ver)" ]; then \
	  echo "Usage: make migrate-force ver=VERSION"; \
	else \
	  $(GO) run $(MAIN) -cmd=force -force=$(ver); \
	fi

.PHONY: migrate-version
migrate-version:
	$(GO) run $(MAIN) -cmd=version

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then \
	  echo "Usage: make migrate-create name=add_users_table"; \
	else \
	  $(GO) run $(MAIN) -cmd=create -name=$(name); \
	fi

.PHONY: db-seed
db-seed:
	$(GO) run $(MAIN) -cmd=seed -env=$(POSTGRES_ENV)

.PHONY: db-seed-custom
db-seed-custom:
	@if [ -z "$(file)" ]; then \
	  echo "Usage: make db-seed-custom file=path/to/file.sql"; \
	else \
	  $(GO) run $(MAIN) -cmd=seed -seed-file=$(file); \
	fi

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: build
build:
	$(GO) build -o db-migrate $(MAIN)

.PHONY: clean
clean:
	rm -f db-migrate