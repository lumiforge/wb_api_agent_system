# WB API Agent System — Runbook

## Local run

```bash
go run ./cmd
````

## Tests

```bash
go test ./...
```

## Required environment variables

### LLM

```bash
export HYDRA_AI_API_KEY="..."
export HYDRA_AI_BASE_URL="..."
export SP_AGENT_MODEL="gpt-4o-mini"
```

`HYDRA_AI_BASE_URL` must point to an OpenAI-compatible API base where `/chat/completions` is available.

### HTTP

```bash
export SP_AGENT_HTTP_ADDR=":8080"
export SP_AGENT_PUBLIC_BASE_URL="http://localhost:8080"
```

### SQLite

```bash
export SP_AGENT_SQLITE_PATH="wb_api_agent_system.db"
```

### WB registry

```bash
export SP_AGENT_WB_REGISTRY_PATH="docs/wb-api"
```

### Prompts

```bash
export SP_AGENT_SYSTEM_PROMPT_PATH="internal/agents/wb_api_agent/prompts/system.md"
export SP_AGENT_PLAN_PROMPT_PATH="internal/agents/wb_api_agent/prompts/plan.md"
export SP_AGENT_EXPLORE_PROMPT_PATH="internal/agents/wb_api_agent/prompts/explore.md"
export SP_AGENT_GENERAL_PROMPT_PATH="internal/agents/wb_api_agent/prompts/general.md"
```

### Database migration

```bash
export SP_AGENT_DATABASE_AUTO_MIGRATE="true"
```

### Debug planner input logging

Default:

```bash
export SP_AGENT_DEBUG_LOG_PLANNER_INPUT="false"
```

Enable only locally:

```bash
SP_AGENT_DEBUG_LOG_PLANNER_INPUT=true go run ./cmd
```

Do not enable full planner input logging in production.

## Startup sequence

When the service starts:

1. config is loaded;
2. SQLite session service is initialized;
3. SQLite registry DB is opened;
4. migration is applied;
5. WB OpenAPI YAML files are loaded from `docs/wb-api`;
6. registry operations are written to SQLite;
7. prompts are read;
8. deterministic planner is created;
9. ADK LLM agent is created;
10. A2A handler is created;
11. HTTP server starts.

Expected startup log contains registry stats:

```text
WB OpenAPI registry loaded: files=... operations=...
```

## Health check

```bash
curl http://localhost:8080/healthz
```

## Registry stats

```bash
curl http://localhost:8080/debug/registry/stats
```

Expected shape:

```json
{
  "total": 296,
  "read": 183,
  "write": 113,
  "unknown_readonly": 0,
  "jam_only": 2,
  "generated_operation_id": 281
}
```

Exact numbers may change when WB OpenAPI YAML files change.

## Registry search debug

```bash
curl 'http://localhost:8080/debug/registry/search?q=Получить%20остатки%20на%20складе%20507%20и%20по%20каждому%20товару%20продажи%20за%20последний%20месяц&readonly_only=true&exclude_jam=true&limit=10'
```

For inventory + sales, expected top operations include:

```text
generated_post_api_v3_stocks_warehouseid
generated_get_api_v1_supplier_sales
```

## A2A request test

```bash
curl -X POST http://localhost:8080/a2a \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": "req_3",
    "method": "message/send",
    "params": {
      "request_id": "req_20260426_000004",
      "marketplace": "wildberries",
      "intent": "get_inventory_and_sales",
      "natural_language_request": "Получить остатки на складе 507 и по каждому товару продажи за последний месяц",
      "entities": {
        "warehouse_id": 507,
        "chrt_ids": [12345678, 87654321]
      },
      "period": {
        "from": "2026-03-26",
        "to": "2026-04-26"
      },
      "constraints": {
        "readonly_only": true,
        "no_jam_subscription": true,
        "max_steps": 10,
        "execution_mode": "automatic"
      }
    }
  }'
```

Expected:

```json
{
  "jsonrpc": "2.0",
  "id": "req_3",
  "result": {
    "status": "ready",
    "warnings": [],
    "validation": {
      "errors": []
    }
  }
}
```

Expected operations:

```text
generated_post_api_v3_stocks_warehouseid
generated_get_api_v1_supplier_sales
```

## Invalid JSON test

```bash
curl -i -X POST http://localhost:8080/a2a \
  -H 'Content-Type: application/json' \
  -d '{bad json'
```

Expected:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32700,
    "message": "parse error"
  }
}
```

## Unknown method test

```bash
curl -i -X POST http://localhost:8080/a2a \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":"x","method":"unknown","params":{}}'
```

Expected:

```json
{
  "jsonrpc": "2.0",
  "id": "x",
  "error": {
    "code": -32601,
    "message": "method not found"
  }
}
```

## Logs

A successful A2A request should log:

```text
a2a message/send finished jsonrpc_id=req_3 request_id=req_20260426_000004 intent=get_inventory_and_sales status=ready duration_ms=...
```

LLM planner input logging should be safe by default:

```text
LLM planner input prepared request_id=... intent=... candidates=... operation_ids=...
```

Full planner input must be logged only when:

```bash
SP_AGENT_DEBUG_LOG_PLANNER_INPUT=true
```

## Common failures

### `adk_planner_execution_failed`

Usually means:

* LLM API is unavailable;
* `HYDRA_AI_BASE_URL` is wrong;
* API key is missing or invalid;
* ADK runner failed.

Check logs.

### `adk_planner_returned_invalid_api_execution_plan`

Usually means:

* LLM returned non-JSON;
* LLM returned wrapped JSON;
* LLM returned wrong shape;
* required fields are missing.

Check raw ADK response logs only in local debug mode.

### `adk_plan_failed_registry_validation`

Means LLM returned a plan that violates registry or executor contract.

Examples:

* operation_id not found;
* method/server/path mismatch;
* unknown query param;
* missing required body field;
* Authorization not bound to executor secret;
* final output references missing step output.

### `needs_clarification`

Means required business input is missing.

Example:

```text
Provide entities.chrt_ids as a non-empty value.
```

## Production checklist

Before production:

```text
go test ./...
```

Check:

```text
/healthz
/debug/registry/stats
/a2a valid request
/a2a invalid JSON
/a2a unknown method
```

Do not enable:

```text
SP_AGENT_DEBUG_LOG_PLANNER_INPUT=true
```

in production.
EOF



## 4. После создания проверь

```bash
go test ./...
```

## Registry embeddings and semantic retrieval

Registry embeddings are stored in a dedicated SQLite database, separate from ADK session storage.

### Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `SP_AGENT_EMBEDDINGS_SQLITE_PATH` | `wb_api_agent_embeddings.db` | SQLite database path for registry operation embeddings |
| `SP_AGENT_EMBEDDING_MODEL` | `text-embedding-3-small` | Embedding model used for registry operation documents and user queries |
| `SP_AGENT_EMBEDDING_DIMENSIONS` | `1536` | Expected vector dimensions |
| `SP_AGENT_EMBEDDING_INDEX_REBUILD_ON_STARTUP` | `false` | Rebuild/catch up registry embeddings during application startup |
| `SP_AGENT_SEMANTIC_RETRIEVAL_ENABLED` | `false` | Enable semantic candidate expansion during registry retrieval |
| `SP_AGENT_SEMANTIC_RETRIEVAL_LIMIT` | `20` | Maximum semantic candidates used for expansion |

### Check embedding index coverage

```bash
curl http://localhost:8080/debug/registry/embeddings/status
```

Expected response shape:

```json
{
  "registry_operations": 420,
  "indexed_embeddings": 420,
  "coverage_ratio": 1,
  "model": "text-embedding-3-small",
  "dimensions": 1536
}
```

This endpoint is read-only. It does not rebuild the index, does not call the embedding provider, and does not expose vectors.

### Rebuild embeddings explicitly

Run the application once with rebuild enabled:

```bash
SP_AGENT_EMBEDDING_INDEX_REBUILD_ON_STARTUP=true go run ./cmd
```

After startup completes, check coverage:

```bash
curl http://localhost:8080/debug/registry/embeddings/status
```

Do not enable semantic retrieval until `coverage_ratio` is close to `1`.

### Enable semantic retrieval

Semantic retrieval only expands registry candidates. It does not replace deterministic ranking and does not become source of truth.

Enable it only after the embedding index is populated:

```bash
SP_AGENT_SEMANTIC_RETRIEVAL_ENABLED=true go run ./cmd
```

Use registry search diagnostics to confirm candidate expansion:

```bash
curl "http://localhost:8080/debug/registry/search?q=остатки%20товаров&limit=10&readonly_only=true&exclude_jam=true"
```

The response should include:

```json
{
  "diagnostics": {
    "lexical_candidates": 10,
    "semantic_candidates": 20,
    "merged_candidates": 25,
    "returned_candidates": 10,
    "semantic_expansion_enabled": true
  }
}
```

### Safe production order

1. Start with semantic retrieval disabled.
2. Rebuild embeddings explicitly.
3. Verify embedding coverage.
4. Enable semantic retrieval.
5. Verify registry search diagnostics.
6. Keep rebuild disabled during normal runtime.



Потом:

```bash
gofmt -w internal/services/a2a/handler.go internal/app/app.go
go test ./...
```