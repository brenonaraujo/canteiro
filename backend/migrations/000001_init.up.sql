CREATE TABLE IF NOT EXISTS schema_info (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO schema_info (key, value) VALUES ('foundation', 'f0')
ON CONFLICT (key) DO NOTHING;
