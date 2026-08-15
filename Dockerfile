# syntax=docker/dockerfile:1

FROM golang:1.25-alpine3.22 AS generate

RUN apk add --no-cache make
WORKDIR /src

COPY go.mod go.sum ./
COPY tools/go.mod tools/go.sum ./tools/
RUN go mod download -modfile=tools/go.mod
COPY Makefile ./
COPY api ./api
RUN make generate

FROM golang:1.25-alpine3.22 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=generate /src/internal/api/generated ./internal/api/generated

RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/politburo ./cmd/politburo

FROM alpine:3.22 AS production

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 65532 politburo \
    && adduser -S -D -H -u 65532 -G politburo politburo

COPY --from=build --chown=65532:65532 /out/politburo /usr/local/bin/politburo

USER 65532:65532
EXPOSE 8082
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --spider --quiet http://127.0.0.1:${PORT:-8082}/health/live || exit 1
ENTRYPOINT ["/usr/local/bin/politburo"]
