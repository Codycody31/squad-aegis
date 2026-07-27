#!/usr/bin/env bash
set -euo pipefail

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for make test-sql" >&2
  exit 1
fi

test_suffix="${PPID}-$$"
postgres_container="squad-aegis-test-pg-${test_suffix}"
clickhouse_container="squad-aegis-test-ch-${test_suffix}"
postgres_password="squad-aegis-test"
clickhouse_password="squad-aegis-test"
clickhouse_image="${CLICKHOUSE_TEST_IMAGE:-clickhouse/clickhouse-server:25.3}"

cleanup() {
  docker rm -f "${postgres_container}" "${clickhouse_container}" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker run --detach --rm \
  --name "${postgres_container}" \
  --env POSTGRES_PASSWORD="${postgres_password}" \
  --publish 127.0.0.1::5432 \
  postgres:18 >/dev/null

docker run --detach --rm \
  --name "${clickhouse_container}" \
  --env CLICKHOUSE_PASSWORD="${clickhouse_password}" \
  --publish 127.0.0.1::9000 \
  "${clickhouse_image}" >/dev/null

for _ in $(seq 1 60); do
  if docker exec "${postgres_container}" pg_isready --username postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "${postgres_container}" pg_isready --username postgres >/dev/null

for database_name in squad_aegis_core_test squad_aegis_workflow_test; do
  docker exec "${postgres_container}" createdb --username postgres "${database_name}"
done

for _ in $(seq 1 90); do
  if docker exec "${clickhouse_container}" clickhouse-client --query "SELECT 1" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "${clickhouse_container}" clickhouse-client --query "SELECT 1" >/dev/null

for migration_file in internal/clickhouse/migrations/*.up.sql; do
  docker exec --interactive "${clickhouse_container}" clickhouse-client --multiquery < "${migration_file}"
done

postgres_port="$(docker port "${postgres_container}" 5432/tcp | sed -n '1s/.*://p')"
clickhouse_port="$(docker port "${clickhouse_container}" 9000/tcp | sed -n '1s/.*://p')"

if [[ -z "${postgres_port}" || -z "${clickhouse_port}" ]]; then
  echo "failed to resolve disposable database ports" >&2
  exit 1
fi

postgres_base="postgres://postgres:${postgres_password}@127.0.0.1:${postgres_port}"
TEST_POSTGRES_DSN="${postgres_base}/postgres?sslmode=disable" \
TEST_POSTGRES_CORE_DSN="${postgres_base}/squad_aegis_core_test?sslmode=disable" \
TEST_POSTGRES_WORKFLOW_DSN="${postgres_base}/squad_aegis_workflow_test?sslmode=disable" \
CLICKHOUSE_TEST_HOST=127.0.0.1 \
CLICKHOUSE_TEST_PORT="${clickhouse_port}" \
CLICKHOUSE_TEST_PASSWORD="${clickhouse_password}" \
go test -p 1 ./internal/db ./internal/core ./internal/workflow_manager ./internal/clickhouse ./internal/identity ./internal/server
