.PHONY: generate generate-mocks migrate migrate-down migrate-status run lint test test-integration

generate:
	go tool oapi-codegen --config oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml

generate-mocks:
	go generate ./internal/usecase/trip/... ./internal/httpserver/trip/...

migrate:
	set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" down

migrate-status:
	set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$${DATABASE_URL}" status

run:
	set -a; . ./.env; set +a; go run ./cmd/trip-service

lint:
	"$$(go env GOPATH)/bin/golangci-lint" run ./...

test:
	go test -race -cover ./...

test-integration:
	set -a; . ./.env; set +a; go test -race -tags=integration ./...
