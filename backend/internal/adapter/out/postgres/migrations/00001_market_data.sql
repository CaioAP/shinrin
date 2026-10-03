-- +goose Up
CREATE TABLE assets (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    market          text    NOT NULL,
    symbol          text    NOT NULL,
    class           text    NOT NULL,
    name            text    NOT NULL DEFAULT '',
    sector          text    NOT NULL DEFAULT '',
    isin            text    NOT NULL DEFAULT '',
    cik             text    NOT NULL DEFAULT '',
    cnpj            text    NOT NULL DEFAULT '',
    is_index_member boolean NOT NULL DEFAULT false,
    active          boolean NOT NULL DEFAULT true,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (market, symbol)
);

-- Raw daily bars. adj_close is the source's own adjusted close (NULL when it
-- has none); analysis re-adjusts from corporate_actions.
CREATE TABLE prices_daily (
    asset_id   bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    date       date   NOT NULL,
    open       double precision NOT NULL,
    high       double precision NOT NULL,
    low        double precision NOT NULL,
    close      double precision NOT NULL,
    adj_close  double precision,
    volume     bigint NOT NULL,
    source     text   NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, date)
);

CREATE TABLE corporate_actions (
    asset_id   bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    ex_date    date   NOT NULL,
    type       text   NOT NULL CHECK (type IN ('dividend', 'jcp', 'split', 'bonus')),
    value      double precision NOT NULL,
    source     text   NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, ex_date, type)
);

-- Long format: one row per reported metric and period.
CREATE TABLE fundamentals (
    asset_id    bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    period_end  date   NOT NULL,
    period_type text   NOT NULL CHECK (period_type IN ('Q', 'FY')),
    metric      text   NOT NULL,
    value       double precision NOT NULL,
    source      text   NOT NULL,
    fetched_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, period_end, period_type, metric)
);

-- Computed indicator snapshots, kept per day for history and backtests.
CREATE TABLE indicators (
    asset_id    bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    as_of       date   NOT NULL,
    name        text   NOT NULL,
    value       double precision NOT NULL,
    computed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, as_of, name)
);

-- +goose Down
DROP TABLE indicators;
DROP TABLE fundamentals;
DROP TABLE corporate_actions;
DROP TABLE prices_daily;
DROP TABLE assets;
