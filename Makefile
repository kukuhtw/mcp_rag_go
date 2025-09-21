# =========================
# MCP PoC — Makefile (Dockerized)
# =========================

DC            ?= docker compose -f deployments/compose/docker-compose.dev.yaml
API_SERVICE   ?= api
WEB_SERVICE   ?= web
MYSQL_SERVICE ?= mysql
DEV_SERVICE   ?= dev
PY_SERVICE    ?= py

# --- load .env jika ada (opsional tapi enak)
ifneq (,$(wildcard .env))
include .env
export
endif

# === DSN ===
# Antar-container (gunakan nama service: mysql)
DSN_DOCKER ?= $(DB_DSN_DOCKER)
ifeq ($(DSN_DOCKER),)
DSN_DOCKER := mcpuser:secret@tcp(mysql:3306)/mcp?parseTime=true&multiStatements=true
endif

# Dari host/laptop (gunakan 127.0.0.1:13306)
DSN_HOST ?= $(DB_DSN_HOST)
ifeq ($(DSN_HOST),)
DSN_HOST := mcpuser:secret@tcp(127.0.0.1:13306)/mcp?parseTime=true&multiStatements=true
endif

START         ?= 2025-09-01 00:00:00
DAYS          ?= 30
EVENTSAVG     ?= 1.5
HSSEAVG       ?= 1.0
WO_COUNT      ?= 50

# --- helper: pastikan mysql reachable dari 'dev' sebelum load ---
ensure-mysql:
	@$(DC) ps --services --filter "status=running" | grep -qx $(MYSQL_SERVICE) || $(DC) up -d $(MYSQL_SERVICE)
	@echo "[wait] checking mysql DNS & port from dev..."
	@$(DC) exec $(DEV_SERVICE) sh -lc '\
	  for i in $$(seq 1 30); do \
	    getent hosts mcp-mysql >/dev/null 2>&1 && nc -z mcp-mysql 3306 >/dev/null 2>&1 && exit 0; \
	    sleep 1; \
	  done; \
	  echo "mysql not reachable from dev after 30s" >&2; exit 1; \
	'


GO_EXPORT := export PATH="$$PATH:/usr/local/go/bin";

# --- Helper: auto-nyalakan dev/py kalau belum Up ---
ensure-dev:
	@$(DC) ps --services --filter "status=running" | grep -qx $(DEV_SERVICE) || $(DC) up -d $(DEV_SERVICE)

ensure-py:
	@$(DC) ps --services --filter "status=running" | grep -qx $(PY_SERVICE) || $(DC) up -d $(PY_SERVICE)

# --- Wait for MySQL to be ready (stabil via socket; 1 definisi saja) ---
# --- Wait for MySQL to be ready ---
wait-for-mysql:
	@echo "Checking MySQL status..."
	@if $(DC) exec -T $(MYSQL_SERVICE) mysql -h127.0.0.1 -uroot -proot -e "SELECT 1" >/dev/null 2>&1; then \
	  echo "MySQL is ready!"; \
	else \
	  echo "MySQL not ready yet, waiting..."; \
	  sleep 5; \
	  if $(DC) exec -T $(MYSQL_SERVICE) mysql -h127.0.0.1 -uroot -proot -e "SELECT 1" >/dev/null 2>&1; then \
	    echo "MySQL is ready!"; \
	  else \
	    echo "ERROR: MySQL still not ready after waiting"; \
	    exit 1; \
	  fi; \
	fi


.PHONY: help up down restart logs sh-api sh-web sh-mysql sh-mysql-host sh-dev sh-py \
        build build-images pull-images \
        migrate seed health \
        gen-data demo-data load-ts load-daily load-events load-hsse load-wo wipe-demo \
        ingest-docs test fmt lint ensure-dev ensure-py wait-for-mysql



# =========================
# Help
# =========================
help:
	@echo ""
	@echo "Dockerized targets:"
	@echo "  up / down / restart     - Start/stop all services (mysql, api, web, dev, py)"
	@echo "  logs                    - Tail logs for all services"
	@echo "  sh-api|sh-web|sh-mysql|sh-dev|sh-py - Shell into a service container"
	@echo "  build-images            - Build images (api)"
	@echo "  gen-data                - Generate CSV sample (via py service)"
	@echo "  demo-data               - gen-data + load all CSVs (via dev service)"
	@echo "  ingest-docs             - Generate embeddings for doc_chunks (via dev)"
	@echo "  test / fmt / lint       - Run inside dev container"
	@echo ""

# =========================
# Lifecycle
# =========================

up:
	$(DC) up -d

down:
	$(DC) down

restart: down up

logs:
	$(DC) logs -f

sh-api:
	$(DC) exec $(API_SERVICE) /bin/sh

sh-web:
	$(DC) exec $(WEB_SERVICE) /bin/sh

sh-mysql:
	$(DC) exec mysql bash -lc 'mysql -hmysql -umcpuser -psecret mcp'

sh-dev:
	$(DC) exec $(DEV_SERVICE) /bin/sh -lc 'bash || sh'

sh-py:
	$(DC) exec $(PY_SERVICE) /bin/sh

# =========================
# Build / Pull
# =========================
build: build-images build-web #migrate 
build-images:
	$(DC) build --no-cache $(API_SERVICE)

pull-images:
	$(DC) pull


# ====== Shell shortcuts ======
sh-mysql:  ## MySQL dari container (pakai service DNS)
	$(DC) exec $(MYSQL_SERVICE) sh -lc 'mysql -hmysql -u$${MYSQL_USER:-mcpuser} -p$${MYSQL_PASSWORD:-secret} $${MYSQL_DATABASE:-mcp}'

sh-mysql-host: ## MySQL dari host (port publish)
	@mysql -h 127.0.0.1 -P 13306 -u$${MYSQL_USER:-mcpuser} -p$${MYSQL_PASSWORD:-secret} $${MYSQL_DATABASE:-mcp}

migrate: wait-for-mysql
	@echo "[migrate] applying SQL migrations via mysql container (inline)..."
	$(DC) exec -T $(MYSQL_SERVICE) bash -lc '\
	  set -euo pipefail; \
	  DB_HOST=127.0.0.1; \
	  DB_USER=root; \
	  DB_PASS="$${MYSQL_ROOT_PASSWORD:-root}"; \
	  DB_NAME="$${MYSQL_DATABASE:-mcp}"; \
	  echo "-> ensure database $$DB_NAME"; \
	  mysql --protocol=TCP -h"$$DB_HOST" -u"$$DB_USER" -p"$$DB_PASS" -e "CREATE DATABASE IF NOT EXISTS \`$$DB_NAME\` DEFAULT CHARACTER SET utf8mb4"; \
	  if [ -f /docker-entrypoint-initdb.d/00-schema.sql ]; then \
	    echo "-> /docker-entrypoint-initdb.d/00-schema.sql"; \
	    mysql --protocol=TCP -h"$$DB_HOST" -u"$$DB_USER" -p"$$DB_PASS" "$$DB_NAME" < /docker-entrypoint-initdb.d/00-schema.sql; \
	  fi; \
	  shopt -s nullglob; \
	  for f in /docker-entrypoint-initdb.d/migrations/*.sql; do \
	    echo "-> $$f"; \
	    mysql --protocol=TCP -h"$$DB_HOST" -u"$$DB_USER" -p"$$DB_PASS" "$$DB_NAME" < "$$f"; \
	  done; \
	  if [ -f /docker-entrypoint-initdb.d/99-seed.sql ]; then \
	    echo "-> /docker-entrypoint-initdb.d/99-seed.sql"; \
	    mysql --protocol=TCP -h"$$DB_HOST" -u"$$DB_USER" -p"$$DB_PASS" "$$DB_NAME" < /docker-entrypoint-initdb.d/99-seed.sql; \
	  fi; \
	  echo "[migrate] done." \
	'

build-web:
	$(DC) build --no-cache $(WEB_SERVICE)
	$(DC) build --no-cache $(API_SERVICE)
	

# ====== Demo Data (pakai PY & DEV) ======
demo-data: gen-data load-po load-ts-signal load-ts load-daily load-events load-wells load-hsse load-wo

gen-data: ensure-py
	$(DC) exec $(PY_SERVICE) sh -lc '\
	  cd tools/gen_dummy && \
	  python3 generate_timeseries.py --start "$(START)" --days $(DAYS) \
	    --events_weekly_avg $(EVENTSAVG) --hsse_daily_avg $(HSSEAVG) \
	    --wo_count $(WO_COUNT) --overwrite \
	'
load-wells: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table wells -csv tools/gen_dummy/sample_wells.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-po: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table purchase_orders -csv tools/gen_dummy/sample_purchase_orders.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-ts: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table ts_value -csv tools/gen_dummy/sample_timeseries.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-ts-signal: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table ts_signal -csv tools/gen_dummy/sample_ts_signal.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-daily: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table prod_allocation_daily -csv tools/gen_dummy/sample_prod_daily.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-events: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table drilling_events -csv tools/gen_dummy/sample_drilling_events.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-hsse: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table hsse_incidents -csv tools/gen_dummy/sample_hsse_incidents.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

load-wo: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table work_orders -csv tools/gen_dummy/sample_work_orders.csv -dsn "$$DSN" -batch 2000 -disable-fk \
	'

# ====== Embedding (opsional) ======
ingest-docs: ensure-dev wait-for-mysql
	$(DC) exec -e DSN="$(DSN_DOCKER)" -e OPENAI_API_KEY \
	  $(DEV_SERVICE) sh -lc '\
	    $(GO_EXPORT) \
	    test -n "$$OPENAI_API_KEY" || { echo "ERROR: OPENAI_API_KEY kosong"; exit 1; } ; \
	    go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	    /tmp/loader -table doc_chunks \
	                -csv tools/gen_dummy/sample_doc_chunks.csv \
	                -dsn "$$DSN" \
	                -batch 1000 \
	                -truncate && \
	    go build -o /tmp/ingest ./cmd/ingest-docs && \
	    /tmp/ingest -dsn "$$DSN" \
	                -batch 128 \
	                -model text-embedding-3-small \
	                -where "embedding IS NULL" \
	  '




wipe-demo: ensure-dev
	$(DC) exec -e DSN="$(DSN_DOCKER)" $(DEV_SERVICE) sh -lc '\
	  $(GO_EXPORT) \
	  go build -o /tmp/loader ./tools/gen_dummy/load_to_mysql.go && \
	  /tmp/loader -table ts_value -csv /dev/null -dsn "$$DSN" -truncate || true && \
	  /tmp/loader -table prod_allocation_daily -csv /dev/null -dsn "$$DSN" -truncate || true && \
	  /tmp/loader -table drilling_events -csv /dev/null -dsn "$$DSN" -truncate || true && \
	  /tmp/loader -table hsse_incidents -csv /dev/null -dsn "$$DSN" -truncate || true && \
	  /tmp/loader -table work_orders -csv /dev/null -dsn "$$DSN" -truncate || true \
	'
test-api:
	@curl -i http://localhost:8080/healthz || true
	@echo
	@curl -iN "http://localhost:8080/chat/stream?q=ping" || true

