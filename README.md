# TTB Label Verification

Prototype tool that helps TTB compliance agents verify alcohol beverage label artwork against Certificate of Label Approval (COLA) application data and mandatory labeling rules (27 CFR parts 4, 5, 7, 16).

A single Go service exposes a JSON REST API and serves the React frontend as static assets. Label text is extracted with a vision model (Anthropic API); pass/fail verification is deterministic Go code. See [docs/SPEC.md](docs/SPEC.md) for requirements and [docs/DECISIONS.md](docs/DECISIONS.md) for architecture decisions.

## Prerequisites

- Go 1.26+
- Node 24+ / npm
- Docker (optional, for containerized runs)
- An Anthropic API key

## Setup

```sh
cp .env.example .env   # then fill in ANTHROPIC_API_KEY
```

## Run with Docker (recommended)

```sh
docker compose up --build
```

App at http://localhost:8080.

## Local development

Backend (serves API + built frontend if present):

```sh
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```

Frontend with hot reload (proxies `/api` to the Go server on :8080):

```sh
cd web
npm install
npm run dev
```

To serve the frontend from the Go binary instead: `cd web && npm run build`, then restart the server.

## Project layout

```
cmd/server/       entrypoint
internal/api/     HTTP handlers and routing
internal/config/  env-var configuration
web/              React + TypeScript + Vite frontend
docs/             spec, decisions, TTB checklists and sample labels
```

## Status

Scaffold stage: health endpoint and static serving only. Extraction, verification engine, and batch processing are the next slices (see docs/DECISIONS.md D4–D7).
