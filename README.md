# Task Management Backend - Architecture & Engineering Decisions

This document details the architectural decisions, design rationale, and implementation choices behind this production-grade Go backend project.

---

## 1. Project Overview and Directory Layout

The project follows a modular, layered clean architecture to strictly enforce separation of concerns:

```text
d:\golang\
├── .github/
│   └── workflows/
│       └── ci.yml                   # Automated GitHub Actions CI pipeline (test, lint, build)
├── cmd/
│   └── app/
│       └── main.go                  # Application bootstrap, DI wiring, and graceful shutdown
├── configs/
│   └── config.go                    # Strongly-typed environment configuration (Postgres, Redis, JWT)
├── deploy/                          # Infrastructure & observability provisioning
│   ├── grafana/
│   │   ├── dashboards/              # Pre-provisioned Task Manager live dashboards
│   │   └── provisioning/            # Automated datasources & dashboard providers
│   └── prometheus/
│       └── prometheus.yml           # Prometheus pull scrape configuration (5s interval)
├── docs/
│   ├── docs.go                      # Embedded OpenAPI byte slice (go:embed)
│   └── openapi.json                 # Complete OpenAPI 3.0.3 specification
├── internal/                        # Private domain and application logic
│   ├── apperrors/
│   │   └── apperror.go              # Centralized typed domain errors with HTTP status mapping
│   ├── dto/
│   │   ├── auth_dto.go              # Auth request DTOs with validation rules and response shapes
│   │   └── task_dto.go              # Task CRUD & batch DTOs with pointer fields and optimistic locking
│   ├── handlers/
│   │   ├── auth_handler.go          # Auth HTTP controllers (register, login, refresh, logout)
│   │   ├── response.go              # Standard JSON response & typed error unwrapping (errors.As)
│   │   ├── swagger_handler.go       # Embedded Swagger UI interface and raw spec provider
│   │   └── task_handler.go          # User-scoped Task HTTP controllers
│   ├── middleware/
│   │   ├── auth_middleware.go       # JWT bearer authentication, revocation check & context injection
│   │   ├── logger.go                # Structured JSON logging via log/slog with duration tracking
│   │   ├── metrics.go               # Prometheus metrics collection middleware
│   │   ├── rate_limiter.go          # Token Bucket IP rate limiter (HTTP 429)
│   │   ├── recovery.go              # Panic recovery middleware with stack traces
│   │   └── request_id.go            # Unique X-Request-ID correlation middleware
│   ├── models/
│   │   ├── task.go                  # Pure domain task entity (versioned with user_id)
│   │   └── user.go                  # Domain user entity (password hash omitted from JSON)
│   ├── repositories/
│   │   ├── task_repository.go       # SQL queries scoped by user_id and optimistic lock verification
│   │   ├── token_repository.go      # Revoked tokens blocklist persistence
│   │   └── user_repository.go       # User persistence queries and unique constraint handling
│   ├── routes/
│   │   ├── auth_routes.go           # Public auth endpoints under /api/v1/auth
│   │   ├── routes.go                # Central router configuration, health checks, metrics, and swagger
│   │   └── task_routes.go           # Protected endpoints under /api/v1/tasks
│   ├── services/
│   │   ├── auth_service.go          # Atomic registration, credential checks, and token issuance
│   │   └── task_service.go          # Business rules, Redis cache-aside, and worker pool batch processing
│   └── workers/
│       ├── task_event_pool.go       # Bounded worker pool for async task lifecycle events
│       └── token_cleanup_worker.go  # Periodic background worker for purging expired tokens
├── pkg/                             # Reusable infrastructure packages
│   ├── auth/
│   │   ├── password.go              # Bcrypt hashing and comparison utilities
│   │   └── token.go                 # JWT Access/Refresh token pair generation & validation
│   ├── cache/
│   │   └── cache.go                 # Redis cache-aside client with resilient NoOp fallback
│   ├── database/
│   │   ├── postgres.go              # Connection pool initialization and ping verification
│   │   ├── schema.go                # Idempotent DDL execution (users, tasks, optimistic locking version)
│   │   └── transactor.go            # Closure-based ACID transaction runner (WithinTransaction)
│   ├── telemetry/
│   │   └── metrics.go               # Prometheus metrics (Counters, Histograms, Gauges, NormalizePath)
│   └── validator/
│       └── validator.go             # Tag-based struct validation engine
├── .dockerignore
├── .env.example                     # Environment template
├── .gitignore                       # Ignored files (binaries, env files)
├── Dockerfile                       # Production 15MB minimal Alpine runner image
├── docker-compose.yml               # Multi-container orchestration (API, Postgres, Redis, Prometheus, Grafana)
├── go.mod                           # Module definitions
└── go.sum                           # Dependency checksums
```

---

## 2. Key Engineering Decisions and Rationale

### Architecture and Layering
* **Layered Clean Architecture:** Code is partitioned into `handlers`, `services`, and `repositories` so that HTTP logic, domain rules, and SQL data access never leak into one another.
* **Constructor-Based Dependency Injection:** Explicit constructors (`New...`) pass dependencies down the tree without hidden global state or reflection magic.
* **Compile-Time Interface Verification:** All repositories and services assert interface implementation at compile time (`var _ Repository = (*PostgresRepository)(nil)`).

### Typed Domain Errors & Error Handling
* **Typed Error Hierarchy (`internal/apperrors`):** Domain errors (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrValidation`) are explicitly typed and carry semantic HTTP status codes.
* **Centralized Response Unwrapping (`internal/handlers/response.go`):** The handler layer unwraps errors using Go standard library `errors.As` and `errors.Is`, guaranteeing uniform JSON error envelopes (`code`, `message`, `details`) with zero brittle string comparisons.

### ACID Transactions & Optimistic Concurrency Control
* **Closure-Based Transactor (`pkg/database/transactor.go`):** Transactions are executed via `WithinTransaction(ctx, func(tx) error)`. If the closure returns `nil`, the transaction commits; if an error or panic occurs, it rolls back automatically.
* **Atomic User Registration:** When a new user registers, the user record and two initial welcome tasks are inserted atomically within a single transaction.
* **Optimistic Concurrency Control (OCC):** The `tasks` table includes a `version INT DEFAULT 1` column. Updates execute `UPDATE tasks SET ..., version = version + 1 WHERE id = $1 AND version = $currentVersion`. If another request modified the task concurrently, zero rows are affected and the service returns a typed `ErrConflict` (HTTP 409 Conflict).

### Authentication & Stateless Authorization
* **Dual-Token Strategy (Access + Refresh):** Short-lived Access Tokens (15m) handle request authorization, while long-lived Refresh Tokens (7d) enable secure rotation.
* **Token Invalidation on Logout (Revocation Blocklist):** Calling `POST /api/v1/auth/logout` records the token hash in PostgreSQL/Redis, immediately blocking revoked tokens from subsequent access.
* **Bcrypt Password Security:** Strong salted bcrypt hashing with JSON exclusion (`json:"-"`) on domain models prevents credential leaks.

### Structured Logging & Security Middleware Pipeline
* **Contextual Correlation (`internal/middleware/request_id.go`):** Generates or propagates `X-Request-ID` across HTTP headers, context, and logs for distributed tracing.
* **Structured JSON Logging (`internal/middleware/logger.go`):** Employs Go's `log/slog` to emit structured JSON logs with duration, status code, remote address, and bytes written.
* **Panic Recovery (`internal/middleware/recovery.go`):** Catches uncaught runtime panics, logs full stack traces to `slog`, and returns a safe HTTP 500 without crashing the server process.
* **Token Bucket IP Rate Limiting (`internal/middleware/rate_limiter.go`):** Enforces rate limits (10 req/s sustained, 20 burst) returning HTTP 429 to mitigate Denial-of-Service and brute-force attacks.

### High-Performance Caching (Redis Cache-Aside)
* **Cache-Aside Pattern (`pkg/cache/cache.go`):** `GetAllTasks` checks Redis first; upon cache misses, it queries PostgreSQL and populates Redis with a 5-minute TTL.
* **Write-Through Invalidation:** Task creation, update, and deletion immediately invalidate the user's cached task list in Redis.
* **Resilient Fallback:** If Redis is offline or unreachable on startup, the application seamlessly falls back to an in-memory `NoOpCache` with zero downtime.

### Full-Stack Observability (Prometheus & Grafana)
* **Custom Prometheus Metrics (`pkg/telemetry/metrics.go`):**
  * `task_manager_http_requests_total`: CounterVec tracking requests partitioned by method, path, and status code.
  * `task_manager_http_request_duration_seconds`: HistogramVec measuring latency into sub-second and multi-second buckets.
  * `task_manager_database_*_connections`: Dynamic gauges bound directly to `db.Stats()` (Open, In-Use, Idle).
* **Cardinality Protection:** `NormalizePath` transforms dynamic URL paths (`/tasks/42` $\rightarrow$ `/tasks/:id`) to prevent time-series memory leaks in Prometheus.
* **Automated Grafana Provisioning (`deploy/grafana/`):** Pre-configured Prometheus datasource and automated JSON dashboard visualizing throughput, status code distributions, and p50/p95/p99 latency quantiles.

### Interactive API Documentation (Swagger / OpenAPI 3.0)
* **Embedded Specification (`docs/docs.go`):** The complete OpenAPI 3.0.3 specification is compiled directly into the binary using `//go:embed`.
* **Interactive Portal (`/swagger/`):** Serves an interactive Swagger UI with JWT Bearer authentication, request duration tracking, and live "Try it out" execution.

### Continuous Integration (GitHub Actions CI)
* **Automated Workflow (`.github/workflows/ci.yml`):**
  * **Test Job:** Runs `go vet`, dependency verification, race detection (`-race`), and generates code coverage summaries.
  * **Lint Job:** Validates canonical formatting across all Go files (`gofmt -l .`).
  * **Build Job:** Cross-compiles the static Linux binary, builds the Alpine Docker image, and validates `docker compose config`.

---

## 3. Environment Configuration

The application loads environment variables from `.env` using `godotenv` with sensible defaults:

| Variable | Description | Example Default |
| :--- | :--- | :--- |
| `PORT` | HTTP server listening port | `8080` |
| `APP_ENV` | Environment mode (`development` / `production`) | `development` |
| `POSTGRES_HOST` | PostgreSQL host address | `127.0.0.1` (or `postgres` in Docker) |
| `POSTGRES_PORT` | PostgreSQL port | `5432` |
| `POSTGRES_USER` | PostgreSQL user | `postgres` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `""` |
| `POSTGRES_DB` | Database name | `golang` |
| `POSTGRES_SSLMODE` | SSL connection mode | `disable` |
| `REDIS_HOST` | Redis host address | `127.0.0.1` (or `redis` in Docker) |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | `""` |
| `JWT_ACCESS_SECRET` | Secret used to sign short-lived access tokens | `your_access_secret` |
| `JWT_REFRESH_SECRET` | Secret used to sign long-lived refresh tokens | `your_refresh_secret` |

---

## 4. API Endpoints Reference

All application endpoints are versioned under `/api/v1`:

### Public & System Endpoints
| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/health` | Health check verifying database connectivity | `200` |
| **GET** | `/metrics` | Prometheus metrics scrape endpoint | `200` |
| **GET** | `/swagger/` | Interactive Swagger UI documentation portal | `200` |
| **GET** | `/swagger/openapi.json` | Raw OpenAPI 3.0.3 specification JSON | `200` |
| **GET** | `/docs` | Redirects to `/swagger/` | `301` |
| **POST** | `/api/v1/auth/register` | Atomically register user and seed welcome tasks | `201`, `400`, `409`, `422` |
| **POST** | `/api/v1/auth/login` | Authenticate credentials and receive token pair | `200`, `400`, `401`, `422` |
| **POST** | `/api/v1/auth/refresh` | Exchange valid refresh token for new access token | `200`, `400`, `401`, `422` |
| **POST** | `/api/v1/auth/logout` | Revoke tokens in blacklist | `200`, `400`, `401` |
| **GET** | `/` | Root greeting endpoint (`/{$}`) | `200` |

### Protected Endpoints (Requires `Authorization: Bearer <access_token>`)
| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/tasks` | List user tasks (optimized via Redis cache-aside) | `200`, `401` |
| **POST** | `/api/v1/tasks` | Create task and invalidate Redis cache | `201`, `400`, `401`, `422` |
| **POST** | `/api/v1/tasks/batch` | Concurrently create tasks via bounded worker pool | `200`, `400`, `401`, `422` |
| **GET** | `/api/v1/tasks/{id}` | Fetch task by ID scoped to authenticated user | `200`, `400`, `401`, `404` |
| **PUT** | `/api/v1/tasks/{id}` | Update task with Optimistic Concurrency Control | `200`, `400`, `401`, `404`, `409` |
| **DELETE**| `/api/v1/tasks/{id}` | Remove task and invalidate cache | `204`, `400`, `401`, `404` |

---

## 5. Running the Application

### Option A: Complete Stack via Docker Compose (Recommended)
Spins up PostgreSQL, Redis, Go API, Prometheus, and Grafana in isolated containers:

```bash
# 1. Prepare environment variables
cp .env.example .env

# 2. Build and launch all 5 containers in the background
docker compose up -d --build

# 3. Check container status
docker compose ps
```

#### Live Access URLs:
* **Task Manager REST API:** [http://localhost:8080](http://localhost:8080)
* **Interactive Swagger UI:** [http://localhost:8080/swagger/](http://localhost:8080/swagger/)
* **Prometheus Metrics Engine:** [http://localhost:9090](http://localhost:9090)
* **Grafana Live Dashboards:** [http://localhost:3000](http://localhost:3000) *(Credentials: `admin` / `admin`)*

---

### Option B: Local Go Development
Requires local PostgreSQL (5432) and Redis (6379) instances:

```bash
# 1. Install dependencies
go mod download

# 2. Run test suite with race detector
go test -v -race ./...

# 3. Start API server
go run cmd/app/main.go
```
