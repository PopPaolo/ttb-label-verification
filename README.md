# TTB Label Verification

Prototype tool that helps TTB compliance agents verify alcohol beverage label artwork against Certificate of Label Approval (COLA) application data and mandatory labeling rules (27 CFR parts 4, 5, 7, 16).

A single Go service exposes a JSON REST API and serves the React frontend as static assets. Label text is extracted with a vision model (Anthropic API); pass/fail verification is deterministic Go code. See [docs/SPEC.md](docs/SPEC.md) for requirements and [docs/DECISIONS.md](docs/DECISIONS.md) for architecture decisions.

**On the agency firewall constraint:** TTB's network blocks outbound traffic to unapproved domains, so the prototype's dependency on the public Anthropic API endpoint would not survive inside the agency network. This is a deliberate prototype-only trade-off: the production path is the same Claude models served through Microsoft Foundry in the agency's own Azure tenancy — an endpoint configuration change, not a model or code change — with a self-hosted vision model behind the same `Extractor` interface as the fully air-gapped fallback. Details in [docs/DECISIONS.md](docs/DECISIONS.md) D4.

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

Core functionality built end to end: vision extraction (D4), deterministic verification engine with CFR-cited rules (D5), single-label and batch APIs (D6/D7), React review UI, and a golden label set with known-good expected reports (`testdata/golden/`, D12). Remaining: live end-to-end run against the Anthropic API, OpenAPI spec (D6), Docker image verification, and deployment (D10).
