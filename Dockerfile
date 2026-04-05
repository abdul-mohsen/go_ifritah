# Multi-stage build
FROM golang:1.21-alpine AS builder

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

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies (with fallback proxy settings)
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOPRIVATE=
ENV CGO_ENABLED=0
RUN go mod download

# Copy Go source files (preserve output.css by not copying static folder)
COPY *.go ./

# Build the application (optimized)
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS="-trimpath -buildvcs=false" \
    go build -a -installsuffix cgo -ldflags="-s -w" -o main .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/main .

# Copy templates
COPY --from=builder /app/templates ./templates

# Copy static files
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/ || exit 1

# Run the application
CMD ["./main"]
