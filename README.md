До production осталось примерно 8 этапов.

## 1. Жёсткая валидация `ApiExecutionPlan`

Сейчас post-validator уже есть, но его нужно усилить:

* проверять все `operation_id` строго по registry;
* запрещать лишние path/query/body поля, которых нет в schema;
* проверять required body/query/path fields;
* проверять `readonly_only`, Jam, secrets;
* проверять `max_steps`;
* проверять, что `ready` plan реально исполняемый;
* для write-операций автоматически ставить `requires_approval=true`.

## 2. Нормальный formatter вместо fallback-логики

Сейчас мы частично достраиваем:

```json
"stocks": "$.stocks"
"sales": "$"
```

Нужно сделать полноценный formatter:

* строить `response_mapping.outputs` из response schema;
* строить `final_output.fields`;
* добавлять transforms: join, aggregate, filter, group;
* уметь возвращать табличный/объектный/summary output.

## 3. Улучшить registry retrieval

Сейчас retrieval уже рабочий, но для прода надо:

* multi-intent ranking: `stocks`, `sales`, `orders`, `warehouses`, `prices`, `reports`;
* cluster-aware candidates: не просто top-N, а top-K на каждый смысловой кластер;
* учитывать method category, token type, subscription requirements;
* сохранять score/debug только в логах;
* возможно добавить FTS5 в SQLite.

## 4. Разделить ADK planner и formatter

Сейчас один `wb_api_planner_agent` делает всё. Лучше сделать цепочку:

```text
retriever
  ↓
planner agent: выбирает операции и строит steps
  ↓
formatter agent/post-processor: приводит к ApiExecutionPlan schema
  ↓
validator
```

Можно оставить один agent на раннем production, но архитектурно лучше разделить.

## 5. Session / memory / compaction

Конфиг для compaction уже есть, но логики ещё нет.

Нужно:

* хранить ADK sessions;
* поддержать follow-up запросы;
* compact history при больших контекстах;
* не хранить секреты;
* ограничить размер tool/registry context.

## 6. A2A compatibility довести до нормального уровня

Сейчас handler минимальный.

Для production нужно:

* нормальный A2A `message/send` payload parsing;
* agent card по актуальной A2A schema;
* task/status lifecycle, если нужно;
* корректные JSON-RPC errors;
* request id / trace id;
* auth на endpoint;
* graceful timeouts.

## 7. Observability и безопасность

Нужно добавить:

* structured logs;
* request_id везде;
* latency метрики;
* registry candidate count;
* LLM latency/error metrics;
* validator error metrics;
* не логировать секреты;
* ограничить размер логируемого planner input;
* rate limit на HTTP endpoint.

## 8. Tests

Минимум перед production:

* unit tests для registry loader;
* tests для `SearchOperations`;
* tests для deterministic planner;
* tests для normalizer;
* tests для validator;
* golden tests для `ApiExecutionPlan`;
* integration test A2A → ready plan;
* tests для blocked/needs_clarification сценариев.

## 9. Executor contract

Сейчас сервис только строит план, HTTP не выполняет. Нужно чётко зафиксировать контракт для будущего executor-а:

* как резолвить `input`;
* как резолвить `executor_secret`;
* как выполнять pagination;
* как применять retry/rate-limit;
* как применять `response_mapping`;
* как применять `transforms`;
* как возвращать final output.

## 10. Production packaging

Нужно:

* Dockerfile;
* env example;
* migration strategy;
* health/readiness endpoints;
* CI;
* versioning;
* README запуска;
* конфигурация model/baseURL/api key;
* отдельный storage path для SQLite.

---

Ближайший практический порядок я бы сделал такой:

```text
1. Усилить PlanValidator
2. Улучшить retrieval до multi-intent ranking
3. Добавить transforms/final_output formatter
4. Добавить tests/golden fixtures
5. Довести A2A handler
6. Добавить observability/security
7. Docker/CI/deploy
```

Самый важный следующий шаг — **усилить PlanValidator**, потому что именно он отделяет “LLM что-то сгенерировала” от “план безопасно можно отдать executor-у”.



------
Нормализовать JSON-RPC ошибки
unknown method;
invalid JSON;
invalid params;
internal error;
method not allowed.
Добавить request size limit
чтобы нельзя было отправить огромный payload.
Добавить context timeout на /a2a
например 60–120 секунд, чтобы ADK/LLM вызов не висел бесконечно.
Добавить request_id в логи
JSON-RPC id;
business request_id;
latency;
result status.
Покрыть handler тестами
/healthz;
unknown method;
valid deterministic request;
invalid JSON;
A2A response shape.