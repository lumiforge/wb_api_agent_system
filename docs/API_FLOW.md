# WB API Agent System — API Flow

## A2A endpoint

Main endpoint:

```text
POST /a2a
````

Protocol:

```text
JSON-RPC 2.0
```

Supported method:

```text
message/send
```

## Request shape

Example:

```json
{
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
}
```

## Response shape

Success response:

```json
{
  "jsonrpc": "2.0",
  "id": "req_3",
  "result": {
    "schema_version": "1.0",
    "request_id": "req_20260426_000004",
    "marketplace": "wildberries",
    "status": "ready",
    "intent": "get_inventory_and_sales",
    "risk_level": "read",
    "requires_approval": false,
    "execution_mode": "automatic",
    "inputs": {},
    "steps": [],
    "transforms": [],
    "final_output": {
      "type": "object",
      "description": "Planned API output.",
      "fields": {}
    },
    "warnings": [],
    "validation": {
      "registry_checked": true,
      "output_schema_checked": true,
      "readonly_policy_checked": true,
      "secrets_policy_checked": true,
      "jam_policy_checked": true,
      "errors": []
    }
  }
}
```

## JSON-RPC errors

Invalid JSON:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32700,
    "message": "parse error"
  }
}
```

Unknown method:

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

Invalid params:

```json
{
  "jsonrpc": "2.0",
  "id": "x",
  "error": {
    "code": -32602,
    "message": "invalid params: ..."
  }
}
```

Internal error:

```json
{
  "jsonrpc": "2.0",
  "id": "x",
  "error": {
    "code": -32603,
    "message": "internal error"
  }
}
```

Timeout:

```json
{
  "jsonrpc": "2.0",
  "id": "x",
  "error": {
    "code": -32000,
    "message": "request timeout"
  }
}
```

## Planning flow

### Step 1 — Parse JSON-RPC

The A2A handler:

* validates method is POST;
* applies request size limit;
* applies request timeout;
* decodes JSON-RPC request;
* validates JSON-RPC version;
* routes `message/send`.

### Step 2 — Parse BusinessRequest

The handler unmarshals `params` into `BusinessRequest`.

Boundary parsing errors return JSON-RPC `-32602`.

### Step 3 — Agent planning

The `wb_api_agent.Agent` receives `BusinessRequest`.

The agent first checks required fields:

* `request_id`;
* `marketplace`;
* `intent`;
* `natural_language_request`.

If required fields are missing, the agent returns `needs_clarification`.

### Step 4 — Deterministic planner

The agent calls deterministic planner first.

If a deterministic scenario matches, the service returns a plan without calling LLM.

Current deterministic scenario:

```text
get_seller_warehouse_stocks
```

### Step 5 — Registry retrieval

If deterministic planner does not handle the request, the agent searches registry operations.

The search uses:

* request intent;
* natural language request;
* entities;
* readonly constraints;
* Jam constraints.

The result is a ranked list of `WBRegistryOperation`.

### Step 6 — PlannerInput

The agent builds `PlannerInput`:

```json
{
  "business_request": {},
  "registry_candidates": [],
  "prompts": {},
  "policies": {},
  "output_contract": "Return exactly one ApiExecutionPlan JSON object..."
}
```

This JSON is sent to ADK LLM agent.

### Step 7 — ADK LLM planning

The ADK LLM agent must return one `ApiExecutionPlan`.

The LLM must use only `registry_candidates`.

The LLM must not execute HTTP.

### Step 8 — Parse and normalize

The agent parses returned JSON.

The normalizer:

* converts camelCase input names to snake_case;
* unwraps nested `InputValue`;
* converts literals to `ValueBinding`;
* ensures period inputs exist.

### Step 9 — Validate

The post-processor validates:

* operation exists in registry;
* method/server/path match registry;
* path/query/body fields match registry schema;
* required bindings are present and non-empty;
* Authorization uses executor secret;
* readonly policy is respected;
* Jam policy is respected;
* final output references existing step outputs.

### Step 10 — Return plan

The final response is returned as JSON-RPC result.

## Debug endpoints

Registry stats:

```text
GET /debug/registry/stats
```

Registry search:

```text
GET /debug/registry/search?q=...&readonly_only=true&exclude_jam=true&limit=10
```

Health:

```text
GET /healthz
```

Agent cards:

```text
GET /.well-known/agent.json
GET /.well-known/agent-card.json
```

## Example curl

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