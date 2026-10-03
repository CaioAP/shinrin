-- +goose Up
-- Factor scores per asset and date (docs/design.md, section 6). details
-- keeps the metrics behind each score so the UI can answer "why".
CREATE TABLE scores (
    asset_id    bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    as_of       date   NOT NULL,
    factor      text   NOT NULL CHECK (factor IN ('valuation', 'quality', 'growth', 'momentum', 'income', 'risk', 'sentiment')),
    value       double precision NOT NULL CHECK (value BETWEEN 0 AND 100),
    details     jsonb  NOT NULL DEFAULT '{}',
    computed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, as_of, factor)
);

-- AI reports with the exact input snapshot the model saw. user_id becomes a
-- foreign key when accounts exist; until then reports are run from the CLI.
CREATE TABLE ai_reports (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        bigint,
    asset_id       bigint REFERENCES assets (id) ON DELETE CASCADE,
    kind           text   NOT NULL CHECK (kind IN ('asset', 'portfolio', 'news_digest')),
    profile        text   NOT NULL,
    as_of          date   NOT NULL,
    input_snapshot jsonb  NOT NULL,
    output         jsonb  NOT NULL,
    omitted        text[] NOT NULL DEFAULT '{}',
    provider       text   NOT NULL,
    model          text   NOT NULL,
    tokens_in      integer NOT NULL,
    tokens_out     integer NOT NULL,
    cost_estimate  double precision,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_reports_asset ON ai_reports (asset_id, created_at DESC);

-- +goose Down
DROP TABLE ai_reports;
DROP TABLE scores;
