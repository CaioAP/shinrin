-- +goose Up
CREATE TABLE quotes_latest (
    asset_id   bigint PRIMARY KEY REFERENCES assets (id) ON DELETE CASCADE,
    price      double precision NOT NULL,
    change_pct double precision NOT NULL,
    as_of      timestamptz NOT NULL,
    source     text NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now()
);

-- Headlines and links only: full article text is the publisher's.
CREATE TABLE news_items (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url          text NOT NULL UNIQUE,
    title        text NOT NULL,
    summary      text NOT NULL DEFAULT '',
    lang         text NOT NULL DEFAULT '',
    source       text NOT NULL,
    published_at timestamptz NOT NULL,
    fetched_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX news_items_published_at ON news_items (published_at DESC);

CREATE TABLE news_assets (
    news_id  bigint NOT NULL REFERENCES news_items (id) ON DELETE CASCADE,
    asset_id bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    PRIMARY KEY (news_id, asset_id)
);
CREATE INDEX news_assets_asset ON news_assets (asset_id);

CREATE TABLE macro_series (
    code       text NOT NULL,
    date       date NOT NULL,
    value      double precision NOT NULL,
    source     text NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (code, date)
);

CREATE TABLE bond_quotes (
    asset_id   bigint NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    date       date   NOT NULL,
    maturity   date   NOT NULL,
    buy_rate   double precision NOT NULL,
    sell_rate  double precision NOT NULL,
    buy_price  double precision NOT NULL,
    sell_price double precision NOT NULL,
    source     text   NOT NULL,
    fetched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (asset_id, date)
);

-- +goose Down
DROP TABLE bond_quotes;
DROP TABLE macro_series;
DROP TABLE news_assets;
DROP TABLE news_items;
DROP TABLE quotes_latest;
