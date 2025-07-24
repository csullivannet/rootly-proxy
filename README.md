# Rootly Proxy

A Go-based reverse proxy for hosting custom status pages at scale. This proxy handles TLS termination, ACME challenges, and routes requests based on hostname lookups stored in PostgreSQL.

## SRE Challenge

This project implements a minimal custom proxy that allows customers to CNAME their status page domains to a centralized proxy service.

## Setup Instructions

### Prerequisites

This project uses Nix flakes for reproducible development environments. Make sure you have:
- Nix with flakes enabled
- Direnv (optional, for automatic environment loading, otherwise you need `nix develop`)
- A docker host, either locally or that you can configure your docker client to connect to.


#### Nix

Nix can be installed by following the [Determinate Systems installer](https://determinate.systems/nix-installer/).


#### Direnv

Direnv is optional. It is available in most system package repositories. Otherwise see the
[installation instructions](https://direnv.net/docs/installation.html).

### Setup

See the Taskfile.yaml for key commands.

Run `task` to get a brief description of each command.

## Test environment

The DB schema simply maps a hostname to a backend URL. A [schema init file is included](init.sql).

The test data includes two customers, acme.com and example.com.

TLS certs are provided with Pebble. Otherwise the server uses a self-signed cert for non-proxied paths.

## Viewing connections

To watch the proxy serve certs and proxy to the backend:

Run `task docker-up && task docker-logs`. In a new terminal run `task test-integration-verbose`.
