# Build stage
FROM golang:1.24.4-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rootly-proxy ./cmd/server

# Final stage
FROM alpine:latest

# Build args for user ID and group ID
ARG UID=1000
ARG GID=1000

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/rootly-proxy .

# Create directory for certificates and set ownership
RUN mkdir -p certs

# Expose ports
EXPOSE 80 443

# Run the binary
CMD ["./rootly-proxy"]