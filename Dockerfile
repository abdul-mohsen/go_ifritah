# Multi-stage build
FROM golang:1.21-alpine AS builder

ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_IMAGE_VERSION=${APP_VERSION}
ARG APP_IMAGE_TAG=dev
ARG APP_IMAGE_COMMIT=${APP_COMMIT}
ARG APP_COMMIT_SHORT
ARG APP_CHANNEL=dev
ARG APP_IMAGE_CHANNEL=${APP_CHANNEL}
ARG APP_SOURCE
ARG APP_CREATED
ARG APP_WORKFLOW_RUN
ARG APP_WORKFLOW_RUN_ID=${APP_WORKFLOW_RUN}
ARG APP_WORKFLOW_RUN_URL
ARG APP_BUILT_AT=${APP_CREATED}
ARG APP_IMAGE_REF
ARG APP_IMAGE_DIGEST
ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_COMMIT}
ENV APP_IMAGE_VERSION=${APP_IMAGE_VERSION}
ENV APP_IMAGE_TAG=${APP_IMAGE_TAG}
ENV APP_IMAGE_COMMIT=${APP_IMAGE_COMMIT}
ENV APP_COMMIT_SHORT=${APP_COMMIT_SHORT}
ENV APP_CHANNEL=${APP_CHANNEL}
ENV APP_IMAGE_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_SOURCE=${APP_SOURCE}
ENV APP_CREATED=${APP_CREATED}
ENV APP_WORKFLOW_RUN=${APP_WORKFLOW_RUN}
ENV APP_WORKFLOW_RUN_ID=${APP_WORKFLOW_RUN_ID}
ENV APP_WORKFLOW_RUN_URL=${APP_WORKFLOW_RUN_URL}
ENV APP_BUILT_AT=${APP_BUILT_AT}
ENV APP_IMAGE_REF=${APP_IMAGE_REF}
ENV APP_IMAGE_DIGEST=${APP_IMAGE_DIGEST}

WORKDIR /app

# Install build dependencies including Node.js for Tailwind and ca-certificates
RUN apk add --no-cache git nodejs npm curl ca-certificates

# Copy package files first for better caching
COPY package.json package.json
COPY tailwind.config.js tailwind.config.js

# Copy static files (includes input.css and existing assets)
COPY static ./static

# Install Tailwind CSS v3 (compatible with npx)
RUN npm install tailwindcss@3.4.0

# Copy templates first for Tailwind content scanning
COPY templates ./templates

# Build Tailwind CSS
RUN npx tailwindcss -i ./static/input.css -o ./static/output.css --minify

RUN set -eu; \
    test -n "$APP_VERSION"; \
    test "$APP_VERSION" != "v0.0.0"; \
    test -n "$APP_IMAGE_TAG"; \
    test "$APP_IMAGE_TAG" != "v0.0.0"; \
    case "$APP_IMAGE_CHANNEL" in dev|release) ;; *) echo "APP_IMAGE_CHANNEL must be dev or release" >&2; exit 1 ;; esac

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies (with fallback proxy settings)
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOPRIVATE=
ENV CGO_ENABLED=0
RUN go mod download

COPY VERSION VERSION

# Copy Go source: top-level files plus all internal packages.
# Previously only `*.go` was copied, which left `config/`, `handlers/`,
# `helpers/`, `middleware/`, `models/` out of the build context, causing
# `package afrita/<pkg> is not in std`.
COPY *.go ./
COPY config ./config
COPY handlers ./handlers
COPY helpers ./helpers
COPY middleware ./middleware
COPY models ./models
COPY resources ./resources

# Build the application (optimized)
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS="-trimpath -buildvcs=false" \
    go build -a -installsuffix cgo -ldflags="-s -w" -o main .

# Final stage
FROM alpine:latest

ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_IMAGE_VERSION=${APP_VERSION}
ARG APP_IMAGE_TAG=dev
ARG APP_IMAGE_COMMIT=${APP_COMMIT}
ARG APP_COMMIT_SHORT
ARG APP_CHANNEL=dev
ARG APP_IMAGE_CHANNEL=${APP_CHANNEL}
ARG APP_SOURCE
ARG APP_CREATED
ARG APP_WORKFLOW_RUN
ARG APP_WORKFLOW_RUN_ID=${APP_WORKFLOW_RUN}
ARG APP_WORKFLOW_RUN_URL
ARG APP_BUILT_AT=${APP_CREATED}
ARG APP_IMAGE_REF
ARG APP_IMAGE_DIGEST
ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_IMAGE_COMMIT}
ENV APP_IMAGE_VERSION=${APP_IMAGE_VERSION}
ENV APP_IMAGE_TAG=${APP_IMAGE_TAG}
ENV APP_IMAGE_COMMIT=${APP_IMAGE_COMMIT}
ENV APP_COMMIT_SHORT=${APP_COMMIT_SHORT}
ENV APP_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_IMAGE_CHANNEL=${APP_IMAGE_CHANNEL}
ENV APP_SOURCE=${APP_SOURCE}
ENV APP_CREATED=${APP_CREATED}
ENV APP_WORKFLOW_RUN=${APP_WORKFLOW_RUN}
ENV APP_WORKFLOW_RUN_ID=${APP_WORKFLOW_RUN_ID}
ENV APP_WORKFLOW_RUN_URL=${APP_WORKFLOW_RUN_URL}
ENV APP_BUILT_AT=${APP_BUILT_AT}
ENV APP_IMAGE_REF=${APP_IMAGE_REF}
ENV APP_IMAGE_DIGEST=${APP_IMAGE_DIGEST}
LABEL org.opencontainers.image.version=${APP_VERSION}
LABEL org.opencontainers.image.revision=${APP_IMAGE_COMMIT}
LABEL org.opencontainers.image.source=${APP_SOURCE}
LABEL org.opencontainers.image.created=${APP_BUILT_AT}
LABEL org.opencontainers.image.ref.name=${APP_IMAGE_TAG}
LABEL org.opencontainers.image.channel=${APP_IMAGE_CHANNEL}
LABEL org.opencontainers.image.workflow.run=${APP_WORKFLOW_RUN_ID}
LABEL org.opencontainers.image.title="ifritah-web"
LABEL com.ifritah.build.version=${APP_VERSION}
LABEL com.ifritah.build.channel=${APP_IMAGE_CHANNEL}
LABEL com.ifritah.build.commit=${APP_IMAGE_COMMIT}
LABEL com.ifritah.build.commit_short=${APP_COMMIT_SHORT}
LABEL com.ifritah.build.tag=${APP_IMAGE_TAG}
LABEL com.ifritah.build.image_tag=${APP_IMAGE_TAG}
LABEL com.ifritah.build.workflow_run_id=${APP_WORKFLOW_RUN_ID}
LABEL com.ifritah.build.workflow_run_url=${APP_WORKFLOW_RUN_URL}
LABEL com.ifritah.build.image_ref=${APP_IMAGE_REF}
LABEL com.ifritah.build.built_at=${APP_BUILT_AT}

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/VERSION ./VERSION

# Copy templates
COPY --from=builder /app/templates ./templates

# Copy static files
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/healthz || exit 1

# Run the application
CMD ["./main"]
