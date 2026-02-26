# ─── Stage 1: Build CSS with Node ─────────────────────────────────────────────
FROM node:20-alpine AS css-builder
WORKDIR /app

# Copy package files
COPY package.json package-lock.json* ./

# Install dependencies
RUN npm ci

# Copy source files for Tailwind
COPY vizburo/ui/input.css ./vizburo/ui/input.css
COPY vizburo/ui/templates ./vizburo/ui/templates
COPY tailwind.config.js ./

# Build CSS
RUN npm run css:build

# ─── Stage 2: Build Go ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /app

# git is needed for go mod
RUN apk add --no-cache git

# download deps
COPY go.mod go.sum ./
RUN go mod download

# copy source & build
COPY . .

# Copy compiled CSS from previous stage
COPY --from=css-builder /app/static/css/output.css ./static/css/output.css

RUN CGO_ENABLED=0 GOOS=linux go build -o bin/app ./cmd/server

# ─── Stage 3: Production ──────────────────────────────────────────────────────
# ─── Stage 3: Production ──────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/bin/app .
# Copy template files for the UI
COPY --from=builder /app/vizburo/ui/templates ./vizburo/ui/templates
COPY --from=builder /app/templates ./templates
# Copy static files
COPY --from=builder /app/static ./static

# expose your port
EXPOSE 8080

# simple healthcheck
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --spider --quiet http://localhost:8080/healthCheck || exit 1

# run the compiled binary
ENTRYPOINT ["./app"]
