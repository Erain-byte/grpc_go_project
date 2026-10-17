BEGIN;

CREATE TABLE IF NOT EXISTS user_identity_index (
    keycloak_subject UUID PRIMARY KEY,
    user_id          BIGINT NOT NULL UNIQUE,
    table_index      SMALLINT NOT NULL CHECK (table_index BETWEEN 0 AND 7),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_identity_index_table
    ON user_identity_index (table_index, user_id);

CREATE TABLE IF NOT EXISTS user_groups (
    id          BIGINT PRIMARY KEY,
    code        VARCHAR(64) NOT NULL UNIQUE,
    name        VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    status      SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups (name);
CREATE INDEX IF NOT EXISTS idx_user_groups_status ON user_groups (status);

CREATE TABLE IF NOT EXISTS user_group_members (
    group_id    BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    table_index SMALLINT NOT NULL CHECK (table_index BETWEEN 0 AND 7),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id),
    CONSTRAINT fk_user_group_members_group
        FOREIGN KEY (group_id) REFERENCES user_groups (id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_group_members_user
    ON user_group_members (user_id);
CREATE INDEX IF NOT EXISTS idx_user_group_members_table
    ON user_group_members (table_index, user_id);
CREATE INDEX IF NOT EXISTS idx_user_group_members_joined
    ON user_group_members (joined_at DESC);

DO $migration$
DECLARE
    shard_index INTEGER;
    suffix      TEXT;
    users_table TEXT;
    wallets_table TEXT;
    transactions_table TEXT;
BEGIN
    FOR shard_index IN 0..7 LOOP
        suffix := lpad(shard_index::TEXT, 2, '0');
        users_table := 'users_' || suffix;
        wallets_table := 'user_wallets_' || suffix;
        transactions_table := 'user_fund_transactions_' || suffix;

        EXECUTE format($sql$
            CREATE TABLE IF NOT EXISTS %I (
                id                BIGINT PRIMARY KEY,
                keycloak_subject  UUID NOT NULL,
                username          VARCHAR(64) NOT NULL,
                email             VARCHAR(255),
                nickname          VARCHAR(128) NOT NULL DEFAULT '',
                avatar_url         VARCHAR(512) NOT NULL DEFAULT '',
                phone              VARCHAR(32),
                status             SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
                last_login_at      TIMESTAMPTZ,
                created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
        $sql$, users_table);

        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (keycloak_subject)',
            'idx_' || users_table || '_subject', users_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (username)',
            'idx_' || users_table || '_username', users_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (email)',
            'idx_' || users_table || '_email', users_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (phone)',
            'idx_' || users_table || '_phone', users_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (status)',
            'idx_' || users_table || '_status', users_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (created_at DESC)',
            'idx_' || users_table || '_created', users_table);

        EXECUTE format($sql$
            CREATE TABLE IF NOT EXISTS %I (
                user_id           BIGINT NOT NULL,
                currency          CHAR(3) NOT NULL,
                available_amount  BIGINT NOT NULL DEFAULT 0 CHECK (available_amount >= 0),
                frozen_amount     BIGINT NOT NULL DEFAULT 0 CHECK (frozen_amount >= 0),
                status            SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),
                version           BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
                created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (user_id, currency)
            )
        $sql$, wallets_table);

        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (status)',
            'idx_' || wallets_table || '_status', wallets_table);

        EXECUTE format($sql$
            CREATE TABLE IF NOT EXISTS %I (
                id               BIGINT PRIMARY KEY,
                user_id          BIGINT NOT NULL,
                currency         CHAR(3) NOT NULL,
                type             SMALLINT NOT NULL CHECK (type IN (1, 2, 3, 4)),
                amount           BIGINT NOT NULL CHECK (amount > 0),
                available_after  BIGINT NOT NULL CHECK (available_after >= 0),
                frozen_after     BIGINT NOT NULL CHECK (frozen_after >= 0),
                business_type    VARCHAR(32) NOT NULL,
                business_id      VARCHAR(128) NOT NULL,
                remark           VARCHAR(512) NOT NULL DEFAULT '',
                created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
        $sql$, transactions_table);

        EXECUTE format('CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I (business_type, business_id)',
            'uk_' || transactions_table || '_business', transactions_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (user_id, created_at DESC, id DESC)',
            'idx_' || transactions_table || '_user_time', transactions_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (currency, created_at DESC)',
            'idx_' || transactions_table || '_currency_time', transactions_table);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (type, created_at DESC)',
            'idx_' || transactions_table || '_type_time', transactions_table);
    END LOOP;
END
$migration$;

COMMIT;

