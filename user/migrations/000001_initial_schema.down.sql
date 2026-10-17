BEGIN;

DO $migration$
DECLARE
    shard_index INTEGER;
    suffix      TEXT;
BEGIN
    FOR shard_index IN REVERSE 7..0 LOOP
        suffix := lpad(shard_index::TEXT, 2, '0');
        EXECUTE format('DROP TABLE IF EXISTS %I', 'user_fund_transactions_' || suffix);
        EXECUTE format('DROP TABLE IF EXISTS %I', 'user_wallets_' || suffix);
        EXECUTE format('DROP TABLE IF EXISTS %I', 'users_' || suffix);
    END LOOP;
END
$migration$;

DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS user_identity_index;

COMMIT;

