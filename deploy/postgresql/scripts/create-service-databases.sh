#!/usr/bin/env bash
set -Eeuo pipefail

# 复用现有 Keycloak PostgreSQL 实例，幂等创建其他服务的独立数据库和账号。
# 本脚本不会删除、重建或修改 keycloak 数据库。

require_password() {
  local variable_name="$1"
  if [[ -z "${!variable_name:-}" ]]; then
    echo "missing required environment variable: ${variable_name}" >&2
    exit 1
  fi
}

create_role_and_database() {
  local role_name="$1"
  local role_password="$2"
  local database_name="$3"

  psql --set ON_ERROR_STOP=1 \
    --username "$POSTGRES_USER" \
    --dbname postgres \
    --set role_name="$role_name" \
    --set role_password="$role_password" \
    --set database_name="$database_name" <<'EOSQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'role_name', :'role_password')
WHERE NOT EXISTS (
  SELECT 1 FROM pg_roles WHERE rolname = :'role_name'
) \gexec

SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'role_name', :'role_password') \gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'database_name', :'role_name')
WHERE NOT EXISTS (
  SELECT 1 FROM pg_database WHERE datname = :'database_name'
) \gexec

SELECT format('ALTER DATABASE %I OWNER TO %I', :'database_name', :'role_name') \gexec
EOSQL
}

require_password ADMIN_DB_PASSWORD
require_password USER_DB_PASSWORD
require_password LLM_DB_PASSWORD
require_password WEB3_DB_PASSWORD

create_role_and_database "admin_service" "$ADMIN_DB_PASSWORD" "admin"
create_role_and_database "user_service" "$USER_DB_PASSWORD" "user_service"
create_role_and_database "llm_service" "$LLM_DB_PASSWORD" "llm_service"
create_role_and_database "web3_service" "$WEB3_DB_PASSWORD" "web3_service"
