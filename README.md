# FinSight AI – Intelligent Invoice & Receipt Automation Platform

FinSight AI is a SaaS platform that automates the ingestion, OCR, and intelligent data extraction of invoices and receipts for small‑and‑medium businesses. It combines on‑premise OCR, optional LLM‑based validation, secure webhook delivery, and a modern dashboard that syncs with popular accounting tools. The system is built for reliability (health checks, queue monitoring) and security (HMAC‑validated webhooks, secret manager integration).

## Technologies

- Go (Gin) – backend API
- PostgreSQL – relational storage
- Redis – caching & rate limiting
- Google Cloud Tasks (or RabbitMQ) – async processing
- Tesseract OCR (or Google Vision) – image text extraction
- Ollama / LiteLLM – optional local LLM for validation
- React + Next.js + TypeScript – frontend
- Tailwind CSS – design system
- Docker & Docker‑Compose – development & deployment
- GitHub Actions – CI/CD with queue‑starvation monitoring
- OpenAPI 3.0 – contract documentation

## Features

- Secure multi‑tenant authentication (JWT + RBAC)
- Drag‑and‑drop invoice/receipt upload with preview
- High‑accuracy OCR extraction (image → structured JSON)
- Optional LLM‑based validation & categorisation (e.g., vendor, tax, line items)
- Async processing queue with exponential back‑off and dead‑letter handling
- Webhook endpoint with constant‑time HMAC validation for downstream systems
- Real‑time dashboard: processing stats, spend charts, error logs
- Export to CSV/JSON and native integrations (QuickBooks, Xero via webhooks)
- Health‑check endpoint (/api/health) exposing DB, queue, OCR, LLM status
- OpenAPI 3.0 spec with auto‑generated Swagger UI
- CI pipeline that detects GitHub Actions queue starvation and alerts
- Audit log of all uploads, edits, and webhook deliveries

## Architecture

FinSight AI follows a modular, container‑first architecture.

- **Frontend (Next.js)** serves a SPA with server‑side rendering for SEO‑critical pages (login, dashboard). It talks to the backend via a typed OpenAPI client.
- **Backend (Go + Gin)** exposes a versioned REST API. Requests pass through auth, logging, and recovery middleware. Core routes include health, auth, invoice CRUD, and webhook management.
- **Processing Pipeline**: Uploaded files are stored in an S3‑compatible bucket (minIO in dev). A message with the file location is pushed to Google Cloud Tasks (or RabbitMQ). A worker service pulls the task, runs OCR, optionally calls the LLM service, persists structured data, and triggers configured webhooks.
- **Data Layer**: PostgreSQL stores users, invoices (metadata + extracted JSON), webhook subscriptions, and audit logs. Migrations are managed with golang‑migrate.
- **Cache**: Redis holds session tokens, rate‑limit counters, and short‑lived OCR results.
- **Security**: Secrets (API keys, DB passwords) are loaded from a YAML config that can be overridden by environment variables or Google Secret Manager. Webhook payloads are signed with HMAC‑SHA256 using a per‑subscription secret.
- **Observability**: Structured logging (Zap), Prometheus metrics endpoint, and the /api/health composite check. GitHub Actions workflow includes a step that queries the Actions queue length; if it exceeds a threshold, a Slack alert is sent.
- **Deployment**: Docker‑Compose for local dev; Helm charts (not listed) for Kubernetes production. All services are stateless, enabling horizontal scaling.
- **API Contract**: OpenAPI 3.0 spec lives in `internal/openapi/openapi.yaml` and is served via Swagger UI at `/docs`.

## Product Design

FinSight AI uses a clean, enterprise‑grade design system built on Tailwind CSS.

- **Typography**: Inter (400/500/600) for body, headings, and UI controls.
- **Color Palette**: Primary #0F62FE, secondary #6F6F6F, success #24A148, danger #DA1E28, background #F4F4F4, surface #FFFFFF, with dark‑mode support via CSS variables.
- **Spacing**: 4‑px base unit, consistent padding/margin using Tailwind's `space-x-` utilities.
- **Components**: Card, Table, Modal, Toast, Avatar, FileUploader, Chart (recharts), and a responsive Sidebar navigation.
- **Interaction States**: Focus rings, hover shadows, disabled opacity, loading spinners.
- **Responsive Layout**: 3‑column layout on desktop, collapsible sidebar on tablet, stacked mobile view with bottom navigation for key actions.
- **Accessibility**: ARIA labels, keyboard navigation, color‑contrast compliance.
- **Theming**: Light/Dark toggle stored in user preferences.
All components are typed with TypeScript and exported from `src/components/` for reuse.

## Testing

Comprehensive testing strategy:

- **Backend**: Go unit tests for services (OCR, LLM, webhook signer) using `testing` and `testify`. Integration tests spin up a PostgreSQL container via `testcontainers-go` to verify DB interactions. End‑to‑end API tests with `httptest` and OpenAPI contract validation.
- **Worker**: Table‑driven tests for queue processing, including failure and dead‑letter scenarios.
- **Frontend**: Jest + React Testing Library for component unit tests. Cypress for end‑to‑end flows (login, upload, dashboard refresh, webhook receipt). Visual regression via Chromatic (optional).
- **CI**: GitHub Actions runs lint, unit, integration, and Cypress tests on each PR. A dedicated job monitors the GitHub Actions queue length; if > 50 pending jobs, it fails the pipeline and posts a Slack alert.
- **Performance**: Load testing of the upload endpoint with k6 scripts (not listed) to ensure scalability.
- **Security**: Static analysis with `gosec` and `npm audit`. HMAC validation tests cover timing‑attack resistance.
All test scripts are invoked from the `scripts/` directory.

## Status

Generated by QuarkForge. Review and test before deployment. Automated test status is recorded in the generation logs.
