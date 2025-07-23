# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

This is a Go-based SRE interview challenge project for implementing a minimal custom proxy. The goal is to create a
reverse proxy with TLS termination.

## Development Setup

Do not rely on the native system having packages available.

This project uses Nix flakes for reproducible development environments. Direnv is used to automatically load the env.

## Development Commands

Once in the Nix development environment:

```bash
# Run the application (once main.go exists)
go run main.go

# Build the application
go build -o rootly-proxy

# Run tests
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

```

The Nix flake provides:
- Go compiler and tools (gopls, gotools, go-tools)
- PostgreSQL
- Development utilities (git, curl, jq, delve)
- TLS tools (openssl)
- Optional Docker support

## Architecture Notes

The proxy will need these main components:

1. **HTTP Server** - Handle HTTPS requests and TLS termination
2. **Database Layer** - PostgreSQL connection and hostname lookup
3. **Proxy Handler** - Fetch content from page_data_url and return to client
4. **ACME Handler** - Handle Let's Encrypt certificate challenges

## Database Setup

The PostgreSQL database should have the `status_pages` table created before running the proxy. Consider including a
migration or setup script for this.
