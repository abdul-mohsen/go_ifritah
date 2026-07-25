#!/usr/bin/env bash
set -euo pipefail

MYSQL_BIN="${MYSQL_BIN:-mysql}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_DATABASE="${MYSQL_DATABASE:-zatca_master}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-}}"

if [[ ! "$MYSQL_DATABASE" =~ ^[A-Za-z0-9_]+$ ]]; then
  printf 'MYSQL_DATABASE must contain only letters, numbers, and underscores\n' >&2
  exit 1
fi

if [[ -n "$MYSQL_PASSWORD" ]]; then
  export MYSQL_PWD="$MYSQL_PASSWORD"
fi

mysql_args=(
  --protocol=TCP
  --host="$MYSQL_HOST"
  --port="$MYSQL_PORT"
  --user="$MYSQL_USER"
  --default-character-set=utf8mb4
)

"$MYSQL_BIN" "${mysql_args[@]}" <<SQL
CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE \`$MYSQL_DATABASE\`;

CREATE TABLE IF NOT EXISTS tenant_plan (
  tenant_name VARCHAR(100) NOT NULL,
  plan ENUM('solo', 'growth', 'business', 'enterprise') NOT NULL DEFAULT 'solo',
  trial_ends_at TIMESTAMP NULL,
  notes VARCHAR(500),
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (tenant_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_feature_override (
  tenant_name VARCHAR(100) NOT NULL,
  feature_id VARCHAR(100) NOT NULL,
  enabled BOOLEAN NOT NULL,
  reason VARCHAR(255),
  expires_at TIMESTAMP NULL,
  PRIMARY KEY (tenant_name, feature_id),
  INDEX idx_tenant (tenant_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE tenant_plan
  ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS notes VARCHAR(500),
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;

ALTER TABLE tenant_feature_override
  ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS reason VARCHAR(255),
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP NULL;
SQL

printf 'Plan tables are ready in %s\n' "$MYSQL_DATABASE"
