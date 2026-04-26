CREATE TABLE IF NOT EXISTS wb_registry_operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    marketplace TEXT NOT NULL,
    source_file TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    method TEXT NOT NULL,
    server_url TEXT NOT NULL,
    path_template TEXT NOT NULL,
    tags_json TEXT NOT NULL DEFAULT '[]',
    category TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    x_readonly_method INTEGER,
    x_category TEXT NOT NULL DEFAULT '',
    x_token_types_json TEXT NOT NULL DEFAULT '[]',
    path_params_schema_json TEXT NOT NULL DEFAULT '{}',
    query_params_schema_json TEXT NOT NULL DEFAULT '{}',
    headers_schema_json TEXT NOT NULL DEFAULT '{}',
    request_body_schema_json TEXT NOT NULL DEFAULT '{}',
    response_schema_json TEXT NOT NULL DEFAULT '{}',
    rate_limit_notes TEXT NOT NULL DEFAULT '',
    subscription_requirements TEXT NOT NULL DEFAULT '',
    max_period_days INTEGER,
    max_lookback_days INTEGER,
    requires_jam INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_file, operation_id, method, server_url, path_template)
);

CREATE INDEX IF NOT EXISTS idx_wb_registry_operations_lookup
ON wb_registry_operations(marketplace, operation_id, method);

CREATE INDEX IF NOT EXISTS idx_wb_registry_operations_path
ON wb_registry_operations(source_file, path_template);