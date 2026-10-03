-- +goose Up
-- Users' own LLM accounts (bring your own key). The key is sealed by the
-- application (AES-256-GCM envelope, bound to the user id); only the last
-- four characters are kept in the clear for display.
CREATE TABLE llm_credentials (
    user_id     bigint  PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    provider    text    NOT NULL,
    model       text    NOT NULL DEFAULT '',
    base_url    text    NOT NULL DEFAULT '',
    key_hint    text    NOT NULL,
    sealed_key  bytea   NOT NULL,
    monthly_cap integer NOT NULL CHECK (monthly_cap > 0),
    updated_at  timestamptz NOT NULL
);

CREATE INDEX ai_reports_user ON ai_reports (user_id, created_at DESC) WHERE user_id IS NOT NULL;

-- +goose Down
DROP INDEX ai_reports_user;
DROP TABLE llm_credentials;
