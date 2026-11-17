CREATE TABLE blocks (
    id         TEXT PRIMARY KEY,
    name       TEXT    NOT NULL,
    position   INT     NOT NULL,
    is_digital BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE sets (
    id                TEXT PRIMARY KEY,
    code              TEXT NOT NULL DEFAULT '',
    name              TEXT NOT NULL,
    block_id          TEXT NOT NULL REFERENCES blocks (id),
    official_count    INT  NOT NULL,
    total_count       INT  NOT NULL,
    release_date      DATE,
    logo_key          TEXT,
    symbol_key        TEXT,
    logo_source_url   TEXT,
    symbol_source_url TEXT
);
CREATE INDEX sets_official_count_idx ON sets (official_count);
CREATE INDEX sets_block_idx ON sets (block_id);

CREATE TABLE cards (
    id                TEXT PRIMARY KEY,
    set_id            TEXT NOT NULL REFERENCES sets (id),
    local_id          TEXT NOT NULL,
    local_number      INT,
    name              TEXT NOT NULL,
    name_search       TEXT NOT NULL,
    rarity            TEXT NOT NULL DEFAULT '',

    source_image_url  TEXT,
    sharp_image_url   TEXT,
    image_key         TEXT,
    thumb_key         TEXT,
    image_source_hash TEXT,

    category          TEXT   NOT NULL DEFAULT '',
    types             TEXT[] NOT NULL DEFAULT '{}',
    hp                INT,
    stage             TEXT   NOT NULL DEFAULT '',
    trainer_type      TEXT   NOT NULL DEFAULT '',
    regulation_mark   TEXT   NOT NULL DEFAULT '',
    illustrator       TEXT   NOT NULL DEFAULT '',

    UNIQUE (set_id, local_id)
);
CREATE INDEX cards_illustrator_idx ON cards (lower(illustrator)) WHERE illustrator <> '';
CREATE INDEX cards_number_idx ON cards (local_number, set_id);
CREATE INDEX cards_name_search_idx ON cards (name_search text_pattern_ops);
CREATE INDEX cards_types_idx ON cards USING GIN (types);

CREATE TABLE card_prices (
    card_id     TEXT PRIMARY KEY REFERENCES cards (id) ON DELETE CASCADE,
    cents       INT NOT NULL CHECK (cents >= 0),
    currency    CHAR(3) NOT NULL DEFAULT 'EUR',
    updated_at  TIMESTAMPTZ NOT NULL,
    avg1_cents  INT CHECK (avg1_cents  >= 0),
    avg7_cents  INT CHECK (avg7_cents  >= 0),
    avg30_cents INT CHECK (avg30_cents >= 0),
    product_id  INT
);
CREATE INDEX card_prices_product_idx ON card_prices (product_id) WHERE product_id IS NOT NULL;

CREATE TABLE card_price_history (
    card_id TEXT NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    day     DATE NOT NULL,
    cents   INT  NOT NULL CHECK (cents >= 0),
    PRIMARY KEY (card_id, day)
);

CREATE TABLE card_foils (
    card_id          TEXT PRIMARY KEY REFERENCES cards (id) ON DELETE CASCADE,
    look             TEXT NOT NULL CHECK (look IN ('holo', 'pillar', 'fullArt', 'ex', 'sir', 'reverse', 'gold')),
    front_key        TEXT,
    mask_key         TEXT,
    etch_key         TEXT,
    front_source_url TEXT,
    mask_source_url  TEXT,
    etch_source_url  TEXT,
    source_hash      TEXT NOT NULL,
    copied_hash      TEXT,
    CONSTRAINT card_foils_has_images CHECK (
        (front_key IS NOT NULL AND mask_key IS NOT NULL) OR (front_source_url IS NOT NULL AND mask_source_url IS NOT NULL))
);

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_key ON users (lower(username));

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE refresh_tokens (
    token_hash BYTEA PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens (session_id);

CREATE TABLE access_tokens (
    token_hash BYTEA PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX access_tokens_session_idx ON access_tokens (session_id);

CREATE TABLE collection_entries (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    card_id   TEXT        NOT NULL REFERENCES cards (id),
    condition TEXT        NOT NULL CHECK (condition IN ('mint', 'nearMint', 'excellent', 'good', 'lightPlayed', 'played', 'poor')),
    language  TEXT        NOT NULL CHECK (language ~ '^[A-Z]{2}$'),
    quantity  INT         NOT NULL CHECK (quantity BETWEEN 1 AND 9999),
    added_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, card_id, condition, language)
);
CREATE INDEX collection_entries_user_added_idx ON collection_entries (user_id, added_at DESC, id);

CREATE TABLE idempotency_keys (
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key          TEXT        NOT NULL CHECK (length(key) BETWEEN 1 AND 128),
    request_hash BYTEA       NOT NULL,
    created      BOOLEAN     NOT NULL,
    entry        JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

CREATE TABLE lists (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL CHECK (kind IN ('favorites', 'trade', 'wishlist', 'custom')),
    name       TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX lists_default_kind_idx ON lists (user_id, kind) WHERE kind <> 'custom';
CREATE INDEX lists_user_idx ON lists (user_id, created_at);

CREATE TABLE list_cards (
    list_id  UUID        NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    card_id  TEXT        NOT NULL REFERENCES cards (id),
    position BIGINT      NOT NULL,
    quantity INT         NOT NULL DEFAULT 1 CHECK (quantity BETWEEN 1 AND 99),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (list_id, card_id)
);
CREATE INDEX list_cards_order_idx ON list_cards (list_id, position);

CREATE FUNCTION create_default_lists() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO lists (user_id, kind, name) VALUES
        (NEW.id, 'favorites', 'Favoris'),
        (NEW.id, 'trade', 'À échanger'),
        (NEW.id, 'wishlist', 'Liste de souhaits');
    RETURN NEW;
END $$;
CREATE TRIGGER users_default_lists AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION create_default_lists();
