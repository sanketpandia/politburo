.PHONY: build run test fmt ci generate generate-api generate-politburo-api generate-infinite-flight-api generate-check openapi-view openapi-stop

GO_TOOL = go tool -modfile=$(CURDIR)/tools/go.mod

build:
	go build -buildvcs=false ./cmd/politburo

run:
	go run ./cmd/politburo

test:
	go test ./...

ci: generate test build

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './internal/api/generated/*')

generate: generate-api

generate-api: generate-politburo-api generate-infinite-flight-api

generate-politburo-api:
	mkdir -p internal/api/generated/politburo
	cd api/openapi && $(GO_TOOL) oapi-codegen -config oapi-codegen.yaml politburo.yaml

generate-infinite-flight-api:
	mkdir -p internal/api/generated/infiniteflight
	cd api/openapi/infinite-flight && $(GO_TOOL) oapi-codegen -config oapi-codegen.yaml openapi.yaml

generate-check: generate

# Generate Go code and start the local read-only Swagger viewer on port 8081.
openapi-view: generate-api
	docker compose -f ../labour-bureau/docker-compose.dev.yml up -d swagger-editor
	@echo "Politburo API: http://localhost:8081"

openapi-stop:
	docker compose -f ../labour-bureau/docker-compose.dev.yml stop swagger-editor
