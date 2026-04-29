Агентская архитектура:

```text
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

Ключевая идея: **LLM больше не строит план целиком**. Он только выбирает операции. Исполняемый `ApiExecutionPlan` строится детерминированным Go-кодом. Это уже отражено в актуализированной архитектуре: selector должен вернуть operation selection result, а не полный executable API plan. 

## 1. `wb_api_agent.Agent` — оркестратор

Это главный агентский процесс. Он не "думает" сам, а управляет пайплайном:

```text
получил BusinessRequest
  ↓
попробовал deterministic planner
  ↓
если не handled — пошёл в registry retrieval
  ↓
передал candidates в ADK selector
  ↓
получил выбранные operation_id
  ↓
зарезолвил их из registry
  ↓
отдал в ApiPlanComposer
  ↓
прогнал post-processing
```

То есть `Agent` — это coordinator между deterministic path, retrieval, LLM selector и composer.

## 2. Deterministic planner — быстрый bypass для известных сценариев

Сначала агент пробует deterministic planner.

```text
BusinessRequest
  ↓
deterministic_planner
  ↓
если сценарий известен → готовый ApiExecutionPlan
  ↓
если нет → handled=false
```

Он нужен для стабильных кейсов, где LLM вообще не нужен. Например текущий known scenario:

```text
get_seller_warehouse_stocks
```

Если deterministic planner справился — ADK selector не вызывается.

## 3. Operation retrieval — подготовка кандидатов для агента

Если deterministic planner не справился, агент ищет подходящие операции в registry.

Retrieval не выбирает финальную операцию. Он только делает bounded candidate set:

```text
BusinessRequest text
  ↓
lexical retrieval
  ↓
optional semantic candidate expansion
  ↓
merge
  ↓
deterministic ranking
  ↓
top registry candidates
```

Semantic retrieval, если включён, только добавляет candidates. Он не принимает решение и не заменяет registry.

## 4. ADK operation selector — единственный LLM-шаг

Это LLM-агент, но его роль сильно ограничена.

Он получает:

```text
BusinessRequest
registry candidates
selection policy
OperationSelectionPlan contract
```

Он должен вернуть не HTTP-план, а что-то уровня:

```json
{
  "status": "selected",
  "selected_operations": [
    {
      "operation_id": "generated_post_api_v3_stocks_warehouseid",
      "purpose": "..."
    }
  ],
  "missing_inputs": []
}
```

Его задача:

```text
понять intent пользователя
  ↓
выбрать operation_id из уже найденных registry candidates
  ↓
сказать, хватает ли бизнес-входов
```

Чего он не должен делать:

```text
строить path/query/body
придумывать operation_id
придумывать headers
придумывать retry/rate limit
создавать ApiExecutionPlan
```

## 5. Registry operation resolver — защита от галлюцинаций selector-а

После selector-а выбранные `operation_id` ещё не считаются истинными.

Resolver делает:

```text
selected operation_id
  ↓
GetOperation из registry source-of-truth
  ↓
если нет такой операции — ошибка/blocked
  ↓
если есть — отдаём полную registry metadata дальше
```

Это важный слой: даже если LLM написал мусорный operation_id, дальше он не пройдёт.

## 6. `ApiPlanComposer` — настоящий строитель плана

Это главный deterministic brain после выбора операции.

Он берёт:

```text
BusinessRequest
resolved registry operations
schema metadata
policy constraints
```

И строит:

```text
ApiExecutionPlan
```

Именно composer решает:

```text
method
server_url
path_template
path_params bindings
query_params bindings
headers
body
content_type
pagination
retry_policy
rate_limit_policy
response_mapping
final_output
validation metadata
```

То есть **LLM выбирает “что вызвать”, composer строит “как вызвать”**.

## 7. `PlanPostProcessor` — финальная нормализация результата

После composer-а план всё равно прогоняется через post-processing:

```text
input normalization
binding normalization
registry identity validation
policy validation
schema validation
response mapping normalization
final output normalization
validation metadata normalization
```

С агентской точки зрения это safety net после построения плана.

## Главная архитектурная граница

Самая важная граница такая:

```text
LLM selector:
  выбирает operation_id

Go composer:
  строит executable plan
```

Это правильная архитектура, потому что LLM используется только там, где он полезен: понять человеческий запрос и сопоставить его с кандидатами. Всё, что должно быть точным, проверяемым и воспроизводимым, делает Go-код.

## Где сейчас слабое место

Слабое место уже проявилось в smoke test:

```text
Покажи остатки товаров на складе 12345 по chrtIds...
```

Selector выбрал clarification, хотя данные были переданы.

Это не надо чинить prompt-алиасами. Правильное место фикса — deterministic normalization/extraction business inputs до selector/composer:

```text
"склад 123 товар 5"
  ↓
warehouse_id = 123
chrt_ids = [5]
```

Тогда selector получает уже нормализованный `BusinessRequest`, а не пытается сам угадывать, что такое “товар 5”.



```text
1. Agent orchestration
2. Retrieval
3. LLM operation selection
4. Operation resolving
5. Plan composition
```

## 1. Главный вход агента

```text
internal/agents/wb_api_agent/agent.go
```

Это главный сценарий:

```text
BusinessRequest
  ↓
Agent.Plan()
  ↓
deterministic planner или LLM path
  ↓
ApiExecutionPlan
```

Смотреть первым. Всё остальное — детали вокруг него.

## 2. Поиск кандидатов операций

```text
internal/services/wb_registry_retrieval/service.go
```

Он отвечает за:

```text
текст запроса
  ↓
registry candidates
```

Рядом лежат детали:

```text
ranking.go              как ранжируются операции
search_tokens.go        как режется запрос
semantic_retriever.go   semantic expansion
embedding_*.go          embeddings/index/search/status
```

Это не “агент”, а подготовка кандидатов для агента.

## 3. LLM выбирает operation_id

```text
internal/agents/wb_api_agent/operation_selector.go
```

Это единственное место, где LLM должен “думать”:

```text
BusinessRequest + candidates
  ↓
selected operation_id / missing inputs
```

Важно: он **не должен строить HTTP-план**.

## 4. Проверка выбранной операции

```text
internal/agents/wb_api_agent/operation_resolver.go
```

Он берёт выбранный LLM-ом `operation_id` и проверяет:

```text
есть ли такая операция в registry?
```

Если нет — LLM не проходит дальше.

## 5. Сборка финального плана

```text
internal/agents/wb_api_agent/api_plan_composer.go
```

Это фактически самый важный файл после `agent.go`.

Он строит:

```text
method
url
path params
query params
headers
body
retry
rate limit
response mapping
final output
```

То есть:

```text
LLM выбрал "что вызвать"
ApiPlanComposer строит "как вызвать"
```

## Что можно игнорировать, пока разбираешь агентский процесс

Временно не трогай:

```text
internal/services/a2a/*
internal/adapters/sqlite/*
internal/adapters/adk/session/*
internal/config/*
cmd/*
```

Они нужны для HTTP, хранения, запуска и wiring, но не объясняют сам агентский процесс.

## Минимальная карта файлов

```text
internal/agents/wb_api_agent/
├── agent.go                  главный pipeline
├── operation_selector.go     LLM выбирает operation_id
├── operation_resolver.go     проверяет operation_id по registry
├── api_plan_composer.go      строит ApiExecutionPlan
├── plan_post_processor.go    финальная нормализация
├── normalizer.go             helpers нормализации
├── validator.go              финальная проверка плана
└── *_test.go                 тесты
```

Retrieval отдельно:

```text
internal/services/wb_registry_retrieval/
├── service.go                главный retrieval service
├── ranking.go                lexical ranking
├── semantic_retriever.go     semantic candidates
├── embedding_indexer.go      rebuild embeddings
├── embedding_search.go       cosine search
└── embedding_status.go       coverage status
```

## Самая короткая ментальная модель

```text
agent.go
  вызывает retrieval service
  вызывает operation_selector
  вызывает operation_resolver
  вызывает api_plan_composer
  вызывает plan_post_processor
```

Если хочешь понять систему — читай именно в таком порядке:

```text
1. internal/agents/wb_api_agent/agent.go
2. internal/agents/wb_api_agent/operation_selector.go
3. internal/agents/wb_api_agent/operation_resolver.go
4. internal/agents/wb_api_agent/api_plan_composer.go
5. internal/services/wb_registry_retrieval/service.go
```
