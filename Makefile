.PHONY: generate generate-mocks migrate migrate-down migrate-status run lint test test-integration

generate:
	go tool oapi-codegen --config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/...

generate-mocks:
	go generate ./internal/...

migrate:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" down

migrate-status:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" status

run:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/trip-service

lint:
	"$$(go env GOPATH)/bin/golangci-lint" run ./...

test:
	go test -race -cover ./...

test-integration:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go test -race -tags=integration ./...
