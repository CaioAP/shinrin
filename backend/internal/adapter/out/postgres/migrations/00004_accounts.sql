-- +goose Up
-- Accounts (docs/design.md, section 10). Emails are stored lowercased by the
-- application, so a plain unique index is enough.
CREATE TABLE users (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email             text   NOT NULL UNIQUE,
    password_hash     text   NOT NULL,
    risk_profile      text   CHECK (risk_profile IN ('conservative', 'moderate', 'aggressive')),
    risk_answers      jsonb,
    risk_updated_at   timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);

-- Only a SHA-256 of each session token is stored.
CREATE TABLE sessions (
    token_hash bytea  PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user ON sessions (user_id);
CREATE INDEX sessions_expiry ON sessions (expires_at);

CREATE TABLE watchlists (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text   NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE TABLE watchlist_items (
    watchlist_id bigint NOT NULL REFERENCES watchlists (id) ON DELETE CASCADE,
    asset_id     bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    added_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (watchlist_id, asset_id)
);

-- Reports now belong to a user; deleting the account deletes them (LGPD).
ALTER TABLE ai_reports
    ADD CONSTRAINT ai_reports_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE ai_reports DROP CONSTRAINT ai_reports_user;
DROP TABLE watchlist_items;
DROP TABLE watchlists;
DROP TABLE sessions;
DROP TABLE users;
