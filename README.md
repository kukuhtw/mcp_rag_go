# MCP Multi Context Protocol — Oil & Gas Prototype

Proyek ini adalah skeleton aplikasi **Multi Context Protocol (MCP)** untuk industri migas. Aplikasi ini dirancang untuk menghubungkan berbagai domain (drilling, production, HSSE, purchase order, timeseries, RAG search) ke dalam satu router protokol, sehingga query dari client bisa diarahkan ke service atau repository yang sesuai.

---

## Struktur Utama

* `cmd/api` → service HTTP (healthz, metrics, SSE chat, login, admin, dsb).
* `cmd/mcp-router` → router intent MCP yang membaca `api/mcp-tools.json` dan `schemas/mcp/*`.
* `cmd/worker` → worker untuk job async / batch.
* `api/openapi.yaml` → definisi OpenAPI endpoint publik.
* `api/mcp-tools.json` → manifest daftar MCP tool yang tersedia.
* `internal/handlers/mcp` → implementasi tool MCP (drilling events, production, PO status, timeseries, dsb).
* `internal/repositories/mysql` → akses data MySQL untuk domain migas.
* `schemas/mcp/*.schema.json` → skema JSON untuk validasi input/output MCP.
* `web/` → frontend React + Vite + Tailwind untuk dashboard & chat SSE.

---

## Quickstart

### 1. Persiapan

* Pastikan **Go 1.22+**, **MySQL 8+**, dan **Node.js 18+** sudah terpasang.
* Copy `.env.example` menjadi `.env` dan sesuaikan konfigurasi (DB, JWT secret, dsb).

### 2. Database

```bash
# Jalankan migrasi
./scripts/migrate.sh

# (opsional) Seed data sample
./scripts/seed.sh
```

### 3. Jalankan Service

```bash
# API utama
go run ./cmd/api

# MCP Router
go run ./cmd/mcp-router

# Worker async
go run ./cmd/worker
```

### 4. Jalankan Frontend

```bash
cd web
npm install
npm run dev
```

Akses di `http://localhost:5173` (default Vite).

---

## Docker & Compose

Untuk menjalankan semua service sekaligus:

```bash
# Development
./scripts/dev_up.sh

# Shutdown
./scripts/dev_down.sh
```

File compose tersedia di `deployments/compose/`.

---

## Logging & Monitoring

* Log aplikasi tersimpan di `logs/`.
* Metrics Prometheus dapat diakses di endpoint `/metrics`.
* Healthcheck endpoint `/healthz`.

---

## Catatan

* Proyek ini masih berupa **Proof of Concept (PoC)**.
* Cocok sebagai referensi arsitektur MCP untuk integrasi multi‑domain di industri migas.




---


## 4) FAQ singkat
- **Kenapa ada spike/drop sesekali?** → Agar demo anomaly detection terasa natural.
- **Bagaimana menambah well/tag baru?** → Edit dict `WELLS` di script Python.
- **Bisa generate jam kerja saja?** → Ubah generator: lompat menit saat di luar jam kerja.
- **Time zone?** → Semua timestamp disimpan UTC (`ts_utc`). Aplikasi bisa mengonversi saat menampilkan.


.
├── .env.example
├── .gitignore
├── Makefile
├── README.md
├── aaa
├── api
│   ├── mcp-tools.json
│   └── openapi.yaml
├── cmd
│   ├── api
│   │   └── main.go
│   ├── mcp-router
│   │   └── main.go
│   └── worker
│       └── main.go
├── configs
│   ├── config.yaml
│   └── logging.yaml
├── db
│   └── mysql
│       ├── migrations
│       │   ├── 0001_init.sql
│       │   ├── 0002_indexes.sql
│       │   └── 0003_sample_data.sql
│       ├── schema.sql
│       └── seed.sql
├── deployments
│   ├── compose
│   │   ├── docker-compose.dev.yaml
│   │   └── docker-compose.prod.yaml
│   └── docker
│       ├── .dockerignore
│       ├── Dockerfile
│       └── Dockerfile.worker
├── go.mod
├── go.sum
├── internal
│   ├── app
│   │   ├── app.go
│   │   └── routes.go
│   ├── config
│   │   └── config.go
│   ├── handlers
│   │   ├── http
│   │   │   ├── admin_handler.go
│   │   │   ├── chat_sse_handler.go
│   │   │   ├── health_handler.go
│   │   │   ├── login_handler.go
│   │   │   └── metrics_handler.go
│   │   └── mcp
│   │       ├── answer_with_docs.go
│   │       ├── detect_anomalies_and_correlate.go
│   │       ├── get_drilling_events.go
│   │       ├── get_po_status.go
│   │       ├── get_production.go
│   │       ├── get_timeseries.go
│   │       ├── rag_search_docs.go
│   │       ├── search_work_orders.go
│   │       └── summarize_npt_events.go
│   ├── logging
│   │   └── logging.go
│   ├── mcp
│   │   ├── protocol.go
│   │   ├── registry.go
│   │   └── router.go
│   ├── middleware
│   │   ├── admin_auth.go
│   │   ├── admin_jwt.go
│   │   ├── auth.go
│   │   ├── rbac.go
│   │   └── request_id.go
│   ├── repositories
│   │   ├── mysql
│   │   │   ├── drilling_repo.go
│   │   │   ├── hsse_repo.go
│   │   │   ├── po_repo.go
│   │   │   ├── production_repo.go
│   │   │   ├── timeseries_repo.go
│   │   │   └── workorders_repo.go
│   │   └── search
│   │       └── rag_repo.go
│   ├── services
│   │   ├── analytics_service.go
│   │   ├── drilling_service.go
│   │   ├── hsse_service.go
│   │   └── production_service.go
│   └── util
│       ├── clock.go
│       ├── errors.go
│       └── ids.go
├── logs
│   ├── .gitkeep
│   ├── access.log
│   ├── app.log
│   ├── audit
│   │   ├── data_access.log
│   │   └── tool_calls.log
│   └── mcp_tools.log
├── pkg
│   ├── db
│   │   └── mysql.go
│   ├── vector
│   │   └── embeddings.go
│   └── weather
│       └── client.go
├── schemas
│   ├── http
│   │   └── responses.schema.json
│   └── mcp
│       ├── tool_answer_with_docs.schema.json
│       ├── tool_detect_anomalies.schema.json
│       ├── tool_get_drilling_events.schema.json
│       ├── tool_get_po_status.schema.json
│       ├── tool_get_production.schema.json
│       ├── tool_get_timeseries.schema.json
│       ├── tool_search_work_orders.schema.json
│       └── tool_summarize_npt.schema.json
├── scripts
│   ├── dev_down.sh
│   ├── dev_up.sh
│   ├── healthcheck.sh
│   ├── migrate.sh
│   └── seed.sh
├── testdata
│   └── fixtures
│       ├── docs_chunks.json
│       ├── drilling_events.json
│       ├── production.json
│       └── timeseries.json
├── tools
│   ├── gen_dummy
│   │   ├── README.md
│   │   ├── generate_timeseries.py
│   │   ├── generate_workorders.py
│   │   ├── load_to_mysql.go
│   │   └── sample
│   │       └── sample_timeseries.csv
│   └── lint
│       └── precommit.sh
├── treemenu
└── web
    ├── .env.example
    ├── index.html
    ├── package.json
    ├── postcss.config.js
    ├── public
    │   └── favicon.svg
    ├── src
    │   ├── App.tsx
    │   ├── components
    │   │   ├── Chart.tsx
    │   │   ├── DataTable.tsx
    │   │   ├── Layout.tsx
    │   │   └── Sidebar.tsx
    │   ├── lib
    │   │   └── api.ts
    │   ├── main.tsx
    │   ├── pages
    │   │   ├── Admin.tsx
    │   │   ├── Chat.tsx
    │   │   ├── Dashboard.tsx
    │   │   ├── Login.tsx
    │   │   └── Timeseries.tsx
    │   ├── routes.tsx
    │   └── styles
    │       └── index.css
    ├── tailwind.config.js
    ├── tsconfig.json
    └── vite.config.ts
