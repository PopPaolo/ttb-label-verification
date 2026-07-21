# DECISIONS — TTB Label Verification Prototype

Technical, architectural, and design decisions for implementing the system described in [SPEC.md](SPEC.md). Each entry records the decision, its status (**Decided** / **Proposed** / **Deferred**), the rationale, and alternatives considered. Decisions trace back to SPEC requirements where relevant.

---

## D1. Overall Architecture — Single-Service Monolith

**Status:** Decided

- One deployable service: an HTTP API that also serves the frontend as static assets. No microservices, no separate frontend host.
- Stateless request handling; all verification state lives in the request/response cycle (see D8).
- **Rationale:** The prototype has one workflow and one team of users. A monolith minimizes deployment surface, keeps the 5-second latency budget free of network hops, and matches the evaluation criterion "appropriate technical choices for the scope."
- **Alternatives:** Separate frontend/backend deployments (more moving parts, CORS config, two deploys for no benefit at this scale).

## D2. Backend Language — Go

**Status:** Decided

- Go for the API server, verification engine, and batch orchestration.
- **Rationale:**
  - Low, predictable latency and small memory footprint serve the hard ~5 s/label budget (SPEC §7.1).
  - First-class concurrency (goroutines + bounded worker pools) maps directly onto the 200–300-label batch requirement (SPEC §6) without external infrastructure.
  - Single static binary → trivial containerization (D9) and fast cold starts on the deployment target (D10).
  - The heavy ML lifting is delegated to an external vision model over HTTP (D4), so Go's thinner ML ecosystem is not a constraint.
- **Alternatives:** Python (richer ML/OCR ecosystem, but slower and heavier for a concurrent API server; ecosystem advantage evaporates once extraction is API-delegated). Node/TypeScript (fine, but Go wins on concurrency ergonomics and deployment weight).

## D3. Frontend — Lightweight SPA (React + TypeScript + Vite)

**Status:** Decided

- Small React + TypeScript app built with Vite, compiled to static assets served by the Go binary.
- Design driven by the usability bar in SPEC §7.2: one primary screen, large obvious controls, drag-and-drop upload, per-field result cards with plain-language pass/flag/fail states. No configuration screens, no navigation depth.
- **Rationale:** React gives componentized result rendering (per-field comparison cards, batch triage table) with minimal ceremony; static build keeps deployment to one artifact.
- **Alternatives:** Server-rendered templates + htmx (viable and even simpler operationally, but interactive result review and batch progress favor a client-side app). A heavy framework (Next.js) is unjustified for a single-page tool.

## D4. Label Extraction — Vision LLM (Claude), Not Traditional OCR

**Status:** Decided

- Label images are sent to the Anthropic API (Claude) as image content with a structured-output schema; the model returns the extracted fields (brand name, class/type, ABV, net contents, name/address, country of origin, government warning text) plus formatting observations (warning casing, apparent bold, separation from other text) and a per-field confidence signal.
- Model: `claude-opus-4-8`, adaptive thinking, low/medium effort (extraction is not deep reasoning), structured outputs (`output_config.format` with a JSON schema) so responses are guaranteed parseable — no free-text parsing layer.
- **Rationale:**
  - TTB labels defeat classic OCR: stylized display fonts, curved/circular text (keg collars), dense small print, multi-piece layouts. A vision LLM reads these and can also report *formatting* attributes (caps, bold, separation) that the warning-statement rules require (SPEC §5.4) — pipeline OCR cannot.
  - One API call per label does extraction end-to-end; structured outputs remove a whole class of parsing bugs.
  - Directly serves the stretch goal of tolerating imperfect photos (SPEC §8) with zero extra engineering.
- **Latency:** a single vision extraction call at low effort typically lands in 2–4 s, inside the 5 s budget. Mitigations if benchmarks miss: reduce image resolution before upload, lower effort, or evaluate a faster model tier — measured, not assumed (see D13).
- **Trade-off accepted:** the prototype calls the public Anthropic API endpoint, which conflicts with TTB's firewall reality (SPEC §7.3 — the vendor pilot failed when the agency firewall blocked the vendor's ML endpoints). Accepted for the prototype because it runs outside the agency network; the production path below is how the same system deploys inside it.
- **Production path — Claude on Microsoft Foundry (Azure):** TTB's cloud platform is Azure (SPEC §7.3). Anthropic serves the same Claude models through Microsoft Foundry (Azure AI Foundry): a Foundry resource provisioned in the agency's own Azure tenancy exposes the same Messages API — including the structured-outputs feature this design depends on — at an endpoint inside the approved network boundary. Migration is an endpoint/auth configuration change, not a model or code-path change: same model, same extraction schema, same latency profile, no dual-quality gap between prototype and production. Items to verify at implementation time (out of prototype scope): which Claude models are enabled on Foundry in the agency's region, FedRAMP authorization status of the Foundry Claude offering, and that the Go client is pointed at the Foundry endpoint (the official Go SDK lacks a dedicated Foundry client today, so this is a base-URL + auth-header seam in `internal/extraction`, kept isolated behind the `Extractor` interface).
- **Fallback for a fully air-gapped requirement:** the `Extractor` interface seam allows a self-hosted VLM/OCR backend (e.g. an Ollama-served open-weights vision model on agency GPU infrastructure) to be substituted without touching the verification engine — at a real cost in extraction quality on stylized/dense label text and in latency on CPU-only hardware. Documented as the escape hatch, not the plan of record.
- **Alternatives:** Tesseract/PaddleOCR (+ layout models) — self-hostable but poor on stylized/curved text and blind to bold/caps semantics; would need a separate matching model anyway. Azure Document Intelligence — Azure-native but weaker at judgment-style extraction and blind to the formatting attributes the warning rules need. Self-hosted open-weights VLM as primary — avoids all external dependency but sacrifices extraction quality and the 5 s SLO on realistic agency hardware; kept as fallback only.

## D5. Verification Engine — Deterministic Go Code; the LLM Extracts, Code Decides

**Status:** Decided

- Comparison of extracted values vs. application data is pure Go: no LLM judgment in the pass/fail path.
- Implements the three matching tiers from SPEC §4.4 as explicit functions:
  - **Exact:** government warning wording/punctuation compared against the 27 CFR Part 16 canonical text; casing checks on "GOVERNMENT WARNING" and Surgeon/General.
  - **Normalized:** case folding, whitespace/punctuation normalization, abbreviation equivalence tables (Alc./Vol. variants, proof↔ABV), unit conversion (mL/L, fl. oz./pint/quart/gallon).
  - **Judgment:** near-matches (e.g., edit-distance within threshold after normalization) produce a *flag* for the agent, never an auto-decision.
- Rule sets per beverage type (wine / malt / distilled spirits) encoded as data-driven rule definitions in Go, mirroring the TTB checklists in SPEC §5 — each rule carries its CFR citation for display.
- **Rationale:** determinism and auditability. An agent (and an evaluator) can ask "why did this fail?" and get a rule ID + citation, not a model's opinion. It also keeps the latency budget: one LLM call per label, everything else microseconds.
- **Alternatives:** letting the LLM do the comparison in the same call — simpler, but non-deterministic pass/fail, harder to test, and impossible to unit-test the regulatory logic in isolation.

## D6. API Contract — REST + JSON, OpenAPI-Documented

**Status:** Decided

- JSON REST API, documented with an OpenAPI spec checked into the repo. Sketch:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/verify` | Single verification: multipart form — label image(s) + application-data JSON. Synchronous; returns the full verification report. |
| `POST` | `/api/batches` | Batch submission: single `.zip` upload containing `manifest.json` + label images (see D6a). Returns a batch ID immediately. |
| `GET` | `/api/batches/{id}` | Batch status + triage summary (counts by pass/flag/fail, per-item statuses). |
| `GET` | `/api/batches/{id}/items/{itemId}` | Full report for one label in a batch (same shape as `/api/verify`). |
| `GET` | `/api/health` | Liveness/readiness. |

- Verification report shape: overall status (`pass` / `needs_review` / `fail`), then per-rule results: `{rule_id, citation, field, application_value, label_value, status, tier, note}`.
- **Rationale:** the single endpoint is synchronous because the 5 s budget makes polling ceremony pointless; batches are asynchronous by nature (D7). OpenAPI doubles as the design artifact for the architecture docs.
- **Alternatives:** gRPC (no browser story without a proxy), GraphQL (overkill for four endpoints).

## D6a. Batch Manifest Format — ZIP + `manifest.json`, Explicit Image References

**Status:** Decided

- A batch submission to `POST /api/batches` is a single `.zip` archive containing:
  - `manifest.json` at the archive root: `{"applications": [ {...} ]}`.
  - Label images, referenced by relative path from the archive root (per-application subfolders recommended, e.g. `app-001/front.jpg`, `app-001/back.jpg`, but not required).
- Each entry in `applications` uses the **same application-data field set as the single `/api/verify` request body** (SPEC §2 — beverage type, brand name, class/type, alcohol content, net contents, name/address, domestic/imported, wine-specific fields, formula number), plus two batch-only fields:
  - `id`: a client-assigned string, unique within the batch — echoed back in batch status/results so the agent's own record-keeping lines up with the report.
  - `images`: an ordered array of archive-relative paths for that application's label image(s) (supports multi-piece labels — brand + back/strip/neck).
- No implicit association by filename pattern or folder order — every image an application uses is listed explicitly in its `images` array. This is the core decision: explicit references over convention-guessing.
- **Validation, at upload time:**
  - Reject the whole batch (fail fast, before any extraction work starts) if: `manifest.json` is missing or fails to parse, any `id` is duplicated, or the archive exceeds a size cap (proposed: 250 MB / 500 applications — tunable via D11 env vars).
  - Per-application, not batch-wide: an `images` entry pointing at a path not present in the archive marks *that application* `failed` with a plain-language reason ("referenced image not found in archive") — the rest of the batch proceeds, consistent with D7's partial-failure semantics.
- **Rationale:** filenames alone can't carry the ~10 structured application fields (ABV, address, appellation, etc.) that verification needs — some out-of-band encoding or a lookup table would be required anyway, which is just a worse manifest. A manifest also generalizes cleanly to multi-piece labels (an array, not a naming scheme like `_front`/`_back` that breaks the moment a label has three pieces). Zip (vs. raw multipart with hundreds of file parts) is the one-file action a drag-and-drop UI needs to stay within the "no hunting for buttons" usability bar (SPEC §7.2), and it's the natural shape for whatever export an importer or the demo-data generator already produces.
- **Size/count caps double as abuse mitigation**: the batch endpoint is unauthenticated and triggers one paid Claude API call per label (D4); without a cap, a public URL accepting arbitrary batch sizes is an open-ended cost vector. The cap is a config value, not a hard architectural limit.
- **Alternatives:** filename convention (e.g. `appID_front.jpg`) — rejected, can't hold full application data and breaks on >2 pieces per label. Per-application multipart fields (one HTTP form per application, submitted as a set) — rejected, unworkable at 200–300 applications from a browser and offers no advantage over a manifest once you're already building structured JSON. Raw multipart with all images as siblings and a separate manifest part — viable but no better than zip and worse for very large part counts in some HTTP stacks; zip was chosen for uniformity of "one file in."

## D7. Batch Processing — In-Process Worker Pool, Async with Polling

**Status:** Decided

- Batch submission returns immediately; a bounded goroutine worker pool (concurrency tuned to API rate limits, ~5–10 parallel extractions) processes items; frontend polls `GET /api/batches/{id}` for progress and triage summary.
- Partial-failure semantics: one unreadable label marks that *item* failed/needs-review; the batch continues.
- **Rationale:** 300 labels × ~3 s serial = 15 minutes; at 8-way parallelism ≈ 2 minutes — acceptable without any queue infrastructure. An external queue (SQS/Rabbit/Redis) adds operational weight the prototype scope explicitly doesn't justify — noted as the natural seam where container orchestration + a real queue would slot in for production, which this project will not reach.
- **Alternatives:** SSE/WebSocket progress push (nicer, deferred — polling is sufficient and simpler); external job queue (production concern, out of scope).

## D8. Persistence — In-Memory Only

**Status:** Decided

- Batch state and reports held in memory with TTL eviction; no database. A process restart loses in-flight batches — acceptable for a prototype and consistent with the no-sensitive-data posture (SPEC §7.4).
- Uploaded images are processed and discarded; nothing is written to durable storage.
- **Rationale:** zero storage = zero retention/PII questions, simplest possible deployment. The store sits behind a small interface so SQLite/Postgres could be introduced if requirements change.
- **Alternatives:** SQLite (adds durability nobody asked for), object storage for images (a retention liability, not a feature, here).

## D9. Packaging — Docker, docker-compose for Local Dev

**Status:** Decided

- Multi-stage Dockerfile: Vite build → Go build → minimal runtime image (distroless/alpine) containing the single binary + static assets.
- `docker-compose.yml` for local development/testing (one service today; the compose file is where a mock-extraction service or future dependencies would attach).
- Container orchestration (Kubernetes etc.) is the acknowledged eventual production shape but explicitly **out of scope** — this project will not get there; the container image is the handoff artifact that keeps that path open.
- **Rationale:** reviewers can `docker compose up` and run everything; the same image deploys to D10 unchanged.

## D10. Deployment Target — Container PaaS

**Status:** Proposed (recommendation: Fly.io or Google Cloud Run)

- Deploy the D9 image to a container PaaS providing a public HTTPS URL, per the deliverable requirement (SPEC §7.3).
- Recommendation: **Fly.io** (simple `fly deploy`, cheap, no cloud-account ceremony) or **Cloud Run** (scale-to-zero, generous free tier). Either satisfies the requirement; pick whichever account already exists.
- Note for the infra-architecture doc: the agency runs Azure, so Azure Container Apps is the analogous production landing zone — worth a mention, not a prototype constraint.

## D11. Configuration & Secrets — Environment Variables

**Status:** Decided

- `ANTHROPIC_API_KEY` and tunables (worker pool size, timeouts, model ID, effort level) via environment variables; `.env.example` in the repo, real secrets never committed.
- Model ID and effort configurable so latency/quality tuning (D13) needs no code change.

## D12. Testing Strategy

**Status:** Decided

- **Verification engine:** pure-function unit tests — the bulk of the test suite. Table-driven Go tests per rule: exact-warning failures (title case, altered wording), normalization equivalences ("STONE'S THROW" vs "Stone's Throw", 32 fl. oz. vs 1 quart), per-beverage conditional rules (table wine ABV exception, malt ABV-only-if).
- **Extraction:** interface-mocked in engine tests; a small integration test suite runs against the real API behind a flag.
- **Golden label set:** curated test labels (TTB samples from `docs/labels/` + AI-generated variants, including deliberate failures) with expected reports — doubles as the demo dataset.

## D13. Latency Verification — Measure, Don't Assume

**Status:** Decided

- The 5-second requirement is treated as a testable SLO: the API logs per-stage timing (upload, extraction call, verification, total) on every request, and the golden set (D12) runs as a latency benchmark.
- Escalation ladder if p95 exceeds budget: downscale image resolution → lower model effort → evaluate faster model tier. Each step is a config change (D11), benchmarked before adoption.

## D14. Observability — Structured Logging Only

**Status:** Decided

- Structured logs (slog) with request IDs and the D13 timing fields. No metrics stack, no tracing — reading logs answers every question a prototype raises. Flagged as the seam where Prometheus/OTel would attach in production.

---

## Deferred / Consciously Not Decided

- **Same-field-of-vision check (distilled spirits):** deferred pending a decision on how container sides are represented at upload (SPEC §11). Prototype likely treats all uploaded images as one field of vision and flags the rule as "verify manually."
- **Type-size verification (mm heights, characters-per-inch):** not verifiable from images of unknown physical scale; reported as "verify manually" with the citation. Revisit only if net-contents-based scale inference proves worthwhile.
- **Authentication:** none for the prototype (public demo URL, no sensitive data). Would be table stakes for anything beyond.
