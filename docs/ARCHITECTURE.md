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
operation retrieval
  ↓
ADK operation selector
  ↓
registry operation resolver
  ↓
ApiPlanComposer
  ↓
PlanPostProcessor
  ↓
ApiExecutionPlan
```

Correlation/technical context is propagated end-to-end as `BusinessRequest.metadata` and returned back in `ApiExecutionPlan.metadata`.

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
* initialize SQLite registry embedding store;
* run database migrations;
* load WB OpenAPI registry;
* create OpenAI-compatible chat model adapter;
* create OpenAI-compatible embedding client;
* create deterministic planner;
* create registry retrieval service;
* optionally create semantic retrieval service;
* optionally rebuild registry embeddings on startup;
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

Important registry retrieval and embeddings flags:

```text
SP_AGENT_EMBEDDINGS_SQLITE_PATH
SP_AGENT_EMBEDDING_MODEL
SP_AGENT_EMBEDDING_DIMENSIONS
SP_AGENT_EMBEDDING_INDEX_REBUILD_ON_STARTUP
SP_AGENT_SEMANTIC_RETRIEVAL_ENABLED
SP_AGENT_SEMANTIC_RETRIEVAL_LIMIT
```

### `internal/services/a2a`

HTTP/A2A boundary.

```text
internal/services/a2a/handler.go
```

Responsibilities:

* expose `/a2a`;
* parse JSON-RPC requests;
* validate boundary-level request shape;
* accept optional `params.metadata` object;
* normalize `request_id` and `metadata.correlation_id` fallback rules;
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

### `internal/services/wb_registry_retrieval`

Registry retrieval and optional semantic candidate expansion.

```text
internal/services/wb_registry_retrieval
```

Responsibilities:

* perform lexical registry retrieval above the raw SQLite store;
* apply readonly and Jam filters;
* rank operation candidates deterministically;
* build stable embedding documents for registry operations;
* rebuild/catch up operation embeddings;
* score persisted embeddings with cosine similarity;
* optionally expand lexical candidates with semantic candidates;
* expose safe retrieval diagnostics;
* expose safe embedding index coverage status.

Semantic retrieval is candidate expansion only. It does not replace deterministic ranking and does not become source of truth.

### `internal/adapters/sqlite`

SQLite infrastructure.

```text
internal/adapters/sqlite
```

Responsibilities:

* open SQLite databases;
* apply migrations;
* store and retrieve WB registry operations;
* provide raw registry operation search;
* store and retrieve registry operation embeddings;
* expose aggregate embedding stats without exposing vectors.

SQLite registry storage is source-of-record storage. Retrieval ranking belongs to `internal/services/wb_registry_retrieval`, not to the SQLite adapter.

### `internal/adapters/adk`

ADK infrastructure adapters.

```text
internal/adapters/adk/session
internal/adapters/adk/llm
```

Responsibilities:

* provide ADK session service over SQLite;
* adapt OpenAI-compatible chat completions API to ADK `model.LLM`;
* adapt OpenAI-compatible embeddings API to the `wbregistry.EmbeddingClient` interface.

The chat model is used for operation selection. The embedding client is used only for registry embedding indexing and semantic candidate expansion.

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
* call registry retrieval when deterministic planner does not handle request;
* build operation selection input;
* run ADK operation selector;
* parse operation selection output;
* resolve selected operations from registry source-of-truth;
* build executable plans through deterministic `ApiPlanComposer`;
* normalize and validate the final plan through `PlanPostProcessor`;
* preserve metadata through deterministic and ADK planning paths;
* emit structured correlation logs (`request_id`, `correlation_id`, `session_id`, `run_id`, `tool_call_id`, `client_execution_id`) at planning milestones.

Important files:

```text
agent.go                    orchestration and ADK execution
operation_selector.go       ADK operation selection
operation_resolver.go       registry operation resolution
api_plan_composer.go        deterministic executable plan construction
planner_input.go            operation selection input contract
normalizer.go               input and binding normalization
validator.go                registry/policy/schema validation
response_formatter.go       response_mapping and final_output defaults
plan_post_processor.go      post-processing entrypoint
callbacks.go                reserved for ADK callbacks
prompts/*.md                agent prompts
```

## Planning flow

### 1. Operation retrieval

The agent searches registry operations through `wb_registry_retrieval.Service`.

Retrieval always includes lexical candidates. If semantic retrieval is enabled, semantic candidates are added before deterministic ranking.

The output of retrieval is a bounded set of registry candidates.

### 2. ADK operation selector

The ADK LLM selector receives the business request and selected registry candidates.

Its job is limited to operation selection:

* choose operation IDs from registry candidates;
* identify whether required business inputs are present;
* ask clarification when required business inputs are missing.

The selector must not build executable HTTP plans.

### 3. Registry operation resolver

The resolver verifies selected operation IDs against registry source-of-truth.

Resolved registry operations are passed to the composer.

### 4. ApiPlanComposer

`ApiPlanComposer` builds the executable `ApiExecutionPlan` deterministically from:

* original `BusinessRequest`;
* selected registry operations;
* registry metadata;
* schema metadata;
* policy constraints.

The composer owns request templates, bindings, retry/rate-limit defaults, response mapping, final output defaults, and validation metadata.

### 5. Post-processing

The generated plan still passes through `PlanPostProcessor`.

The post-processor normalizes and validates the final plan before returning it to the client.

## Registry retrieval

Registry retrieval has two layers:

```text
BusinessRequest text
  ↓
lexical retrieval
  ↓
optional semantic candidate expansion
  ↓
candidate merge by operation_id
  ↓
deterministic ranking
  ↓
registry candidates for operation selection
```

### Lexical retrieval

Lexical retrieval is always enabled.

```text
BusinessRequest text
  ↓
tokens + aliases
  ↓
SQLite raw search
  ↓
readonly/Jam filters
  ↓
pre-limit candidates
  ↓
Go scoring
  ↓
multi-intent selection
```

The lexical layer uses:

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

### Semantic candidate expansion

Semantic retrieval is optional and disabled by default.

```text
BusinessRequest text
  ↓
query embedding
  ↓
persisted operation embeddings
  ↓
cosine similarity
  ↓
top semantic operation_id candidates
  ↓
GetOperation from registry source-of-record
  ↓
readonly/Jam filters
  ↓
merge with lexical candidates
```

Semantic retrieval only expands the candidate set. It does not:

* choose the final operation;
* build API plans;
* replace registry metadata;
* bypass policy filters;
* bypass deterministic ranking.

Final ordering still goes through deterministic `rankOperations`.

### Embedding index

Registry operation embeddings are stored in a dedicated SQLite database, separate from ADK session storage and separate from the main registry store.

Each operation embedding is keyed by:

```text
operation_id
model
dimensions
content_hash
```

The embedding document is built deterministically from registry operation metadata. `content_hash` allows rebuild/catch-up indexing to skip unchanged operations.

Embedding rebuild is explicit opt-in:

```text
SP_AGENT_EMBEDDING_INDEX_REBUILD_ON_STARTUP=true
```

Semantic candidate expansion is also explicit opt-in:

```text
SP_AGENT_SEMANTIC_RETRIEVAL_ENABLED=true
```

Normal runtime should keep embedding rebuild disabled.

### Retrieval diagnostics

`/debug/registry/search` returns safe diagnostics:

```json
{
  "diagnostics": {
    "lexical_candidates": 80,
    "semantic_candidates": 16,
    "merged_candidates": 95,
    "returned_candidates": 10,
    "semantic_expansion_enabled": true
  }
}
```

Diagnostics do not expose vectors or model internals.

### Embedding index status

`/debug/registry/embeddings/status` returns aggregate coverage:

```json
{
  "registry_operations": 296,
  "indexed_embeddings": 296,
  "coverage_ratio": 1,
  "model": "text-embedding-3-small",
  "dimensions": 1536
}
```

This endpoint is read-only. It does not rebuild embeddings, does not call the embedding provider, and does not expose vectors.

## LLM input

The LLM does not receive the whole registry and does not read YAML files directly.

The operation selector receives:

* original `BusinessRequest`;
* selected registry candidates;
* operation selection policy;
* output contract for `OperationSelectionPlan`.

The LLM must return an operation selection result, not a full executable API plan.

Executable plan construction is owned by deterministic Go code in `ApiPlanComposer`.

## Post-processing

Generated plans are never trusted directly.

`PlanPostProcessor` performs:

1. input normalization;
2. binding normalization;
3. registry identity validation;
4. metadata source-of-truth enforcement;
5. policy validation;
6. path/query/body schema validation;
7. response mapping normalization;
8. final output normalization;
9. validation metadata normalization.

Important metadata rules:

* LLM-generated metadata is not trusted;
* metadata is not used to choose WB operations;
* metadata is not used in path/query/body bindings;
* returned plan metadata always comes from the input `BusinessRequest`.

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

## Debug endpoints

Health check:

```text
GET /healthz
```

Registry stats:

```text
GET /debug/registry/stats
```

Registry search diagnostics:

```text
GET /debug/registry/search?q=остатки%20товаров&limit=10&readonly_only=true&exclude_jam=true
```

Registry embeddings status:

```text
GET /debug/registry/embeddings/status
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
* registry loader;
* SQLite registry store;
* SQLite embedding store;
* registry lexical retrieval;
* registry multi-intent ranking;
* optional semantic candidate expansion;
* embedding document builder;
* embedding indexer;
* embedding vector search;
* embedding index status;
* OpenAI-compatible embedding adapter through `httptest`;
* ADK operation selector boundary;
* registry operation resolver;
* ApiPlanComposer;
* PlanPostProcessor;
* PlanValidator;
* A2A handler boundary behavior;
* registry debug diagnostics endpoints.

Run:

```bash
go test ./...
```

## Runtime smoke test

Default runtime, semantic retrieval disabled:

```bash
go run ./cmd
```

Check registry and embeddings status:

```bash
curl -s http://localhost:8090/healthz
curl -s http://localhost:8090/debug/registry/stats
curl -s http://localhost:8090/debug/registry/embeddings/status
curl -s "http://localhost:8090/debug/registry/search?q=остатки%20товаров&limit=10&readonly_only=true&exclude_jam=true"
```

Explicit embedding rebuild:

```bash
SP_AGENT_EMBEDDING_INDEX_REBUILD_ON_STARTUP=true go run ./cmd
```

Semantic retrieval runtime:

```bash
SP_AGENT_SEMANTIC_RETRIEVAL_ENABLED=true go run ./cmd
```

Semantic retrieval should only be enabled after embedding coverage is close to `1`.

## Architectural rules

* `app.go` wires dependencies only.
* `services/a2a` handles HTTP only.
* `services/deterministic_planner` contains deterministic planning only.
* `services/wb_registry_retrieval` owns retrieval ranking and semantic candidate expansion.
* `adapters/sqlite` provides raw storage/search only; ranking must not live in SQLite adapters.
* `agents/wb_api_agent` owns ADK/LLM orchestration.
* Domain packages must not depend on infrastructure.
* External API and DB access must go through adapters.
* LLM output must always pass post-processing before returning to client.
* SQLite registry adapters provide raw storage/search only; ranking belongs to `services/wb_registry_retrieval`.
* Semantic retrieval may only expand candidates; it must not replace registry source-of-truth.
* Embedding vectors must not be returned from debug endpoints.
* Embedding rebuild must stay explicit opt-in.
* LLM operation selection must not construct executable HTTP request templates.
* `ApiPlanComposer` owns executable plan construction.