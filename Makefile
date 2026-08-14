.PHONY: build run test fmt generate generate-api generate-check

build:
	go build -buildvcs=false ./cmd/politburo

run:
	go run ./cmd/politburo

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './internal/api/generated/*')

generate: generate-api

generate-api:
	cd api/openapi/health && go tool oapi-codegen -config oapi-codegen.yaml openapi.yaml

generate-check: generate
	git diff --exit-code -- internal/api/generated

