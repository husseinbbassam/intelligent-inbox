# Intelligent Inbox — Self-Evolving AI Data Inbox

A production-ready Go microservice that ingests data from multiple source systems, stores it in PostgreSQL, and uses an Azure OpenAI "Judge" to filter contextual noise — even when the JSON structure is syntactically valid.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Clean Architecture                       │
│                                                                 │
│  cmd/server/        Application entry point & DI wiring        │
│  internal/domain/   Domain models + repository/AIJudge interfaces│
│  internal/repository/  GORM-backed Postgres implementations     │
│  internal/usecase/  Business logic (ingestion, AI judge, feedback)│
│  internal/infrastructure/  Azure OpenAI AIJudge implementation  │
│  internal/api/      Echo HTTP handlers + router                 │
└─────────────────────────────────────────────────────────────────┘
```

The `AIJudge` is defined as an interface in the domain layer (`internal/domain/interfaces.go`), meaning the concrete LLM provider (currently Azure OpenAI) can be swapped for any other provider (OpenAI, Anthropic, a local model, etc.) by implementing that single interface — no business logic changes needed.

## The Self-Learning Loop (Human-in-the-Loop)

```
                  ┌──────────────┐
   Source System  │   /ingest    │  Raw JSON stored as PENDING
   ─────────────► │   endpoint   │ ─────────────────────────────►
                  └──────────────┘                               │
                                                                 ▼
                  ┌──────────────────────────────────────────────┐
                  │            AI Judge Worker (async)           │
                  │                                              │
                  │  1. Fetch PENDING records                    │
                  │  2. Query last 5 human corrections for the   │
                  │     same SystemID (Few-Shot examples)        │
                  │  3. Build dynamic prompt:                    │
                  │       ## Context (human corrections)         │
                  │       ## Record to Evaluate                  │
                  │  4. Call Azure OpenAI → score + CLEAN/NOISE  │
                  │  5. On 401/429/500 → mark PENDING_RETRY,     │
                  │     log warning, continue pipeline           │
                  └──────────────────────────────────────────────┘
                         │                    │
                   CLEAN ▼              NOISE ▼
                  ┌────────────┐   ┌────────────────┐
                  │ Downstream │   │ Human Review   │
                  │ processing │   │ Queue          │
                  └────────────┘   └───────┬────────┘
                                           │
                              Human corrects wrong label
                                           │
                                           ▼
                                  POST /api/v1/feedback
                                  { correct_label: "CLEAN",
                                    correction_note: "..." }
                                           │
                                           ▼
                                  Stored in human_feedback table
                                           │
                            Next AI evaluation for this SystemID
                            includes this correction as a
                            Few-Shot example → AI learns
```

### Why This Improves Over Time

Every time a human corrects an AI mistake via the `/feedback` endpoint, that correction is stored in the `human_feedback` table. The next time the AI Judge evaluates a record from the same source system, it fetches the **last 5 corrections** and prepends them to the prompt as labeled examples. This "in-context learning" loop means the AI continuously calibrates to the organisation's specific definition of noise — without retraining.

## Supported Source Systems

| System ID | Schema | Key Noise Signals |
|-----------|--------|-------------------|
| `procurement` | `invoice_id`, `vendor_name`, `total_amount`, `description` | Gibberish vendor name, negative total, description of only special characters |
| `iot` | `device_id`, `sensor_type`, `value`, `timestamp` | Impossible sensor value, random device IDs |
| `system3` | `source_key`, `data` (arbitrary JSON) | Placeholder for future integration |
| `system4` | `source_key`, `data` (arbitrary JSON) | Placeholder for future integration |

## Database Schema

```sql
-- Stores all incoming records with AI judgment fields
CREATE TABLE ingested_records (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id    TEXT NOT NULL,
    raw_data     JSONB NOT NULL,
    ai_score     FLOAT,          -- 0.0 (pure noise) → 1.0 (perfectly clean)
    ai_status    TEXT DEFAULT 'PENDING', -- PENDING | PENDING_RETRY | CLEAN | NOISE
    ai_reasoning TEXT,
    created_at   TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ
);

-- Stores human corrections used as Few-Shot examples
CREATE TABLE human_feedbacks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    system_id       TEXT NOT NULL,
    record_id       UUID,
    original_data   JSONB,
    correct_label   TEXT NOT NULL,  -- CLEAN | NOISE
    correction_note TEXT,
    created_at      TIMESTAMPTZ
);
```

## API Reference

### Ingest a Record

```
POST /api/v1/ingest/:systemId
Content-Type: application/json

# Example — Procurement Invoice (CLEAN)
curl -X POST http://localhost:8080/api/v1/ingest/procurement \
  -H 'Content-Type: application/json' \
  -d '{
    "invoice_id": "INV-2024-001",
    "vendor_name": "Acme Supplies Ltd",
    "total_amount": 4500.00,
    "description": "Office furniture Q1 2024"
  }'

# Example — IoT Telemetry (likely NOISE — negative temperature)
curl -X POST http://localhost:8080/api/v1/ingest/iot \
  -H 'Content-Type: application/json' \
  -d '{
    "device_id": "SENSOR-042",
    "sensor_type": "temperature",
    "value": -9999.0,
    "timestamp": "2024-01-15T10:30:00Z"
  }'
```

**Response:** `202 Accepted` with the created record (status = `PENDING`).

### List Records

```
GET /api/v1/records?system_id=procurement&status=NOISE&limit=20&offset=0
```

### Get a Single Record

```
GET /api/v1/records/:id
```

### Submit Human Feedback

```
POST /api/v1/feedback
Content-Type: application/json

{
  "record_id": "<uuid-of-the-record>",
  "correct_label": "CLEAN",
  "correction_note": "Vendor name is unusual but valid — internal test vendor"
}
```

## Quick Start

### 1. Start Postgres

```bash
docker compose up -d postgres
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env and fill in your Azure OpenAI credentials
```

### 3. Run the Server

```bash
# Load .env manually or use a tool like direnv
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```

The server starts on port `8080` (configurable via `PORT` env var).
The AI Judge worker starts automatically in the background and polls every **10 seconds** for `PENDING` records.

## Configuration

| Environment Variable | Required | Description |
|---------------------|----------|-------------|
| `POSTGRES_DSN` | ✅ | PostgreSQL connection string |
| `AZURE_OPENAI_ENDPOINT` | ✅ | Azure OpenAI resource endpoint URL |
| `AZURE_OPENAI_API_KEY` | ✅ | Azure OpenAI API key |
| `AZURE_OPENAI_DEPLOYMENT` | ✅ | Azure OpenAI deployment name (e.g. `gpt-4o`) |
| `PORT` | ❌ | HTTP server port (default: `8080`) |

## Tech Stack

- **Go 1.24+** — language
- **[Echo v4](https://echo.labstack.com/)** — HTTP framework
- **[GORM](https://gorm.io/)** — ORM (auto-migrates schema on startup)
- **[pgx v5](https://github.com/jackc/pgx)** — Postgres driver
- **[Azure OpenAI SDK for Go](https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/ai/azopenai)** — LLM provider
- **PostgreSQL 16** — persistence

## Fallback & Resilience

The AI Judge worker handles Azure OpenAI failures gracefully:

| HTTP Status | Behaviour |
|-------------|-----------|
| `401 Unauthorized` | Mark record `PENDING_RETRY`, log warning, continue |
| `429 Too Many Requests` | Mark record `PENDING_RETRY`, log warning, continue |
| `500 Internal Server Error` | Mark record `PENDING_RETRY`, log warning, continue |
| Parse error | Log warning, skip record (remains `PENDING` for next cycle) |
| Any other error | Log warning, skip record |

`PENDING_RETRY` records are currently visible in the API but not automatically re-queued. A future enhancement could add a scheduled retry sweep that moves `PENDING_RETRY` records back to `PENDING` after a cooldown period.

## Swapping the AI Provider

To replace Azure OpenAI with a different LLM (e.g. standard OpenAI, Anthropic Claude, or a local Ollama model), implement the `domain.AIJudge` interface:

```go
// internal/domain/interfaces.go
type AIJudge interface {
    Judge(ctx context.Context, systemID SystemID, rawData []byte, fewShots []*HumanFeedback) (*JudgeResult, error)
}
```

Then pass your new implementation to `usecase.NewAIJudgeUseCase(...)` in `cmd/server/main.go`. No other code changes are required.