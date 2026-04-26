# WB API Agent System — Architecture

## Purpose

WB API Agent System builds validated execution plans for Wildberries API requests.

The service receives a business-level request through an A2A-compatible JSON-RPC HTTP endpoint and returns an `ApiExecutionPlan`. The service does not execute Wildberries HTTP requests itself. It only plans them.

## High-level flow

```text
Client
  ↓
A2A JSON-RPC /a2a
  ↓
BusinessRequest
  ↓
wb_api_agent.Agent
  ↓
deterministic planner
  ↓ if not handled
registry search
  ↓
ADK LLM planner
  ↓
PlanPostProcessor
  ↓
ApiExecutionPlan
```

## Source of API truth

The source of truth for available Wildberries API operations is:

```text
docs/wb-api/*.yaml
```

At application startup, the registry loader parses OpenAPI YAML files and stores normalized operation metadata in SQLite.

The LLM does not read YAML files directly. It only receives selected registry candidates prepared by the application.

## Main packages

### `cmd`

Application entrypoint.

```text
cmd/main.go
```

Responsibilities:

* load config;
* create application;
* handle OS shutdown signals;
* run HTTP server.

### `internal/app`

Dependency wiring.

```text
internal/app/app.go
```

Responsibilities:

* initialize SQLite session service;
* initialize SQLite registry store;
* run database migration;
* load WB OpenAPI registry;
* read prompt files;
* create deterministic planner;
* create ADK-backed WB API agent;
* create A2A handler;
* register HTTP routes.

This package wires dependencies only. Business logic should not live here.

### `internal/config`

Runtime configuration.

```text
internal/config/config.go
```

Responsibilities:

* read environment variables;
* expose immutable application config.

### `internal/services/a2a`

HTTP/A2A boundary.

```text
internal/services/a2a/handler.go
```

Responsibilities:

* expose `/a2a`;
* parse JSON-RPC requests;
* validate boundary-level request shape;
* enforce request timeout;
* enforce max request size;
* return JSON-RPC responses/errors;
* expose debug registry endpoints;
* expose health endpoint and agent card.

This layer must not contain planning logic.

### `internal/services/wb_registry`

OpenAPI registry loader.

```text
internal/services/wb_registry/loader.go
```

Responsibilities:

* parse WB OpenAPI YAML files;
* extract operations;
* normalize metadata;
* persist operations into registry store.

### `internal/adapters/sqlite`

SQLite infrastructure.

```text
internal/adapters/sqlite
```

Responsibilities:

* open SQLite DB;
* apply migrations;
* store and retrieve WB registry operations;
* search operations with lexical retrieval, scoring, and multi-intent ranking.

### `internal/adapters/adk`

ADK infrastructure adapters.

```text
internal/adapters/adk/session
internal/adapters/adk/llm
```

Responsibilities:

* provide ADK session service over SQLite;
* adapt OpenAI-compatible chat completions API to ADK `model.LLM`.

### `internal/services/deterministic_planner`

Code-based deterministic scenarios.

```text
internal/services/deterministic_planner
```

Responsibilities:

* handle known safe scenarios without LLM;
* build deterministic `ApiExecutionPlan`;
* return `handled=false` when request is not a known scenario.

Current deterministic scenario:

```text
get_seller_warehouse_stocks
```

### `internal/agents/wb_api_agent`

Main agent orchestration.

```text
internal/agents/wb_api_agent
```

Responsibilities:

* validate required business request fields;
* call deterministic planner first;
* call registry search when deterministic planner does not handle request;
* build `PlannerInput`;
* run ADK LLM agent;
* parse LLM output;
* normalize and validate the final plan through `PlanPostProcessor`.

Important files:

```text
agent.go                 orchestration and ADK execution
planner_input.go         LLM input contract
normalizer.go            input and binding normalization
validator.go             registry/policy/schema validation
response_formatter.go    response_mapping and final_output defaults
plan_post_processor.go   post-processing entrypoint
callbacks.go             reserved for ADK callbacks
prompts/*.md             agent prompts
```

## Planning modes

### 1. Deterministic planning

Used for known, stable, high-confidence scenarios.

Example:

```text
intent=get_seller_warehouse_stocks
```

The deterministic planner builds a plan directly from request entities and registry operation metadata.

Advantages:

* predictable;
* fast;
* no LLM cost;
* easy to test.

### 2. ADK LLM fallback

Used when deterministic planner does not handle the request.

The agent:

1. searches registry operations;
2. prepares `PlannerInput`;
3. sends `PlannerInput` to ADK LLM agent;
4. parses returned JSON;
5. validates and normalizes the plan.

The LLM is constrained by:

* registry candidates only;
* readonly policy;
* Jam subscription policy;
* no HTTP execution policy;
* no real secrets policy;
* post-validation against registry.

## Registry search

Registry search is lexical, not embedding-based.

Current retrieval strategy:

```text
BusinessRequest text
  ↓
tokens + aliases
  ↓
SQLite LIKE search
  ↓
readonly/Jam filters
  ↓
pre-limit candidates
  ↓
Go scoring
  ↓
multi-intent selection
  ↓
registry candidates for LLM
```

There are no embeddings, vector search, pgvector, FAISS, or cosine similarity.

The search uses:

* substring matching;
* aliases;
* weighted field scoring;
* business relevance boosts;
* multi-intent ranking.

Multi-intent ranking ensures that compound requests like "остатки + продажи" include strong candidates for each active cluster.

Current intent clusters:

```text
stocks
sales
orders
warehouses
```

## LLM input

The LLM receives `PlannerInput`, not the whole registry.

`PlannerInput` contains:

* original `BusinessRequest`;
* selected `RegistryCandidates`;
* prompts;
* planning policies;
* output contract.

The LLM must return exactly one `ApiExecutionPlan` JSON object.

## Post-processing

The LLM output is never trusted directly.

`PlanPostProcessor` performs:

1. input normalization;
2. binding normalization;
3. registry identity validation;
4. policy validation;
5. path/query/body schema validation;
6. response mapping normalization;
7. final output normalization.

If the plan is invalid, the service returns a blocked or needs-clarification `ApiExecutionPlan`.

## Security model

The planner must not:

* execute WB HTTP requests;
* return real secrets;
* invent operation IDs;
* use operations outside registry candidates;
* bypass readonly policy;
* bypass Jam subscription policy.

Authorization headers inside plans must use:

```json
{
  "source": "executor_secret",
  "secret_name": "WB_AUTHORIZATION",
  "required": true
}
```

## Current stable scenario

The current tested end-to-end scenario:

```text
intent=get_inventory_and_sales
request: Получить остатки на складе 507 и по каждому товару продажи за последний месяц
entities:
  warehouse_id
  chrt_ids
period:
  from
  to
```

Expected plan:

```text
step 1: generated_post_api_v3_stocks_warehouseid
step 2: generated_get_api_v1_supplier_sales
```

Expected status:

```text
ready
```

## Testing baseline

Current tests cover:

* deterministic planner;
* registry multi-intent ranking;
* ADK fallback normalization;
* PlanPostProcessor;
* PlanValidator;
* A2A handler boundary behavior.

Run:

```bash
go test ./...
```

## Architectural rules

* `app.go` wires dependencies only.
* `services/a2a` handles HTTP only.
* `services/deterministic_planner` contains deterministic planning only.
* `agents/wb_api_agent` owns ADK/LLM orchestration.
* Domain packages must not depend on infrastructure.
* External API and DB access must go through adapters.
* LLM output must always pass post-processing before returning to client.