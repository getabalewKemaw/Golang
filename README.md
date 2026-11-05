# Task Management Backend - Architecture & Engineering Decisions

This document details the architectural decisions, design rationale, and implementation choices behind this Go backend project.

---

## 1. Project Overview and Directory Layout

The project follows a modular, layered structure to strictly enforce separation of concerns:

```text
d:\golang\
├── cmd/
│   └── app/
│       └── main.go                  # Application bootstrap and explicit dependency wiring
├── configs/
│   └── config.go                # Strongly-typed environment configuration (DB, JWT secrets)
├── internal/                    # Private domain and application logic
│   ├── dto/
│   │   ├── auth_dto.go          # Auth request DTOs with validation rules and response shapes
│   │   └── task_dto.go          # Task request DTOs with pointer fields for partial updates
│   ├── handlers/
│   │   ├── auth_handler.go      # Auth HTTP controllers (register, login, refresh, logout)
│   │   └── task_handler.go      # Task HTTP controllers (user-scoped CRUD)
│   ├── middleware/
│   │   └── auth_middleware.go   # JWT bearer authentication, revocation check & context injection
│   ├── models/
│   │   ├── task.go              # Pure domain task entity (linked with user_id)
│   │   └── user.go              # Domain user entity (password hash omitted from JSON)
│   ├── repositories/
│   │   ├── task_repository.go  # SQL queries scoped by user_id and repository interface
│   │   ├── token_repository.go # Revoked tokens blocklist persistence
│   │   └── user_repository.go  # User persistence queries and unique constraint handling
│   ├── routes/
│   │   ├── auth_routes.go       # Public auth endpoints under /api/v1/auth
│   │   ├── routes.go            # Central router configuration and health checks
│   │   └── task_routes.go       # Protected endpoints under /api/v1/tasks
│   └── services/
│       ├── auth_service.go      # Credential checks, password hashing, and token issuance
│       └── task_service.go      # Business rules and user-scoped task validation
├── pkg/                         # Reusable infrastructure packages
│   ├── auth/
│   │   ├── password.go          # Bcrypt hashing and comparison utilities
│   │   └── token.go             # JWT Access/Refresh token pair generation & validation
│   ├── database/
│   │   ├── postgres.go          # Connection pool initialization and ping verification
│   │   └── schema.go            # Idempotent DDL execution (users, tasks, foreign keys)
│   └── validator/
│       └── validator.go         # Tag-based struct validation engine
├── .env                         # Local environment settings
├── .env.example                 # Example environment template
├── .gitignore                   # Ignored files (binaries, env files)
├── go.mod                       # Module definitions
└── go.sum                       # Dependency checksums
```

---

## 2. Key Engineering Decisions and Rationale

### Architecture and Layering
* **Layered Clean Architecture:** I separated the code into `handlers`, `services`, and `repositories` so that HTTP logic, business rules, and SQL data access never leak into one another. If we migrate from PostgreSQL to another storage engine, only the repository layer changes.
* **Constructor-Based Dependency Injection:** I avoided global state, init functions with side-effects, and reflection-based containers. Each component accepts its dependencies via an explicit constructor (`New...`), passing dependencies from the database pool up to the router in `main.go`.
* **Interface-Driven Repositories:** I defined `TaskRepository`, `UserRepository`, and `TokenRepository` as interfaces that concrete PostgreSQL structs implement. This completely decouples business services from database drivers and makes unit testing straightforward with mocks.
* **Pure Controllers (`internal/handlers`) vs. Routes (`internal/routes`):** I extracted route registration out of the handlers and into a dedicated `routes` package. Handlers act strictly as controllers, while URL paths, `/api/v1` versioning, and middleware chains are configured centrally.

### Authentication & Stateless Authorization
* **Dual-Token Stateless Strategy (Access + Refresh):** I implemented short-lived Access Tokens (15 minutes) for API request authorization and longer-lived Refresh Tokens (7 days) for rotating credentials. This minimizes the security blast radius if an access token is intercepted while preserving smooth user sessions.
* **Token Type Claims Guard & Explicit Expiration Verification:** I added an explicit `token_type` claim (`access` vs `refresh`) and enforced verification inside `ValidateToken()`. Furthermore, beyond standard `jwt.ParseWithClaims()` validation, I included an explicit check (`time.Now().After(claims.ExpiresAt.Time)`) to ensure expired tokens are categorically rejected with `ErrExpiredToken`.
* **Token Invalidation on Logout (Revocation Blocklist):** To enable true logout in a stateless system, I created an idempotent `revoked_tokens` table in PostgreSQL. When calling `POST /api/v1/auth/logout`, the SHA-256 hash of the presented access token and/or refresh token is persisted alongside its expiration timestamp. Middleware and refresh endpoints verify revocation, immediately blocking revoked tokens until their natural expiration.
* **Distinct Cryptographic Signing Secrets:** I configured separate secrets for access tokens and refresh tokens (`JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET`) so that compromising one key does not compromise the entire authentication pipeline.
* **Bcrypt Password Security:** I hashed passwords with `bcrypt` using standard cost factors. Bcrypt incorporates salt generation and intentional computational work to defend against rainbow table and brute-force attacks.
* **JSON Leak Prevention on Domain Models:** I marked the `PasswordHash` field in `models.User` with `json:"-"`. This guarantees that password hashes are omitted if user structs are ever passed to JSON encoders.
* **Collision-Resistant Context Keys:** In `internal/middleware/auth_middleware.go`, I defined unexported type-safe context keys (`type contextKey string`) rather than built-in strings to store authenticated user identifiers in `request.Context()`. This prevents key collisions across packages.
* **Struct Embedding for Claims:** In `pkg/auth/token.go`, I embedded `jwt.RegisteredClaims` inside `Claims`. This leverages Go's composition to promote all standard JWT fields and methods into our custom struct without code duplication.

### Database and Data Isolation
* **Relational Foreign Key Linking:** I linked `tasks` to `users` using `user_id BIGINT REFERENCES users(id) ON DELETE CASCADE`. If a user account is removed, all associated tasks are cleaned up automatically.
* **Multi-Tenant User Scoping:** In both `TaskRepository` and `TaskService`, I scoped all CRUD queries with `WHERE user_id = $1`. When fetching or modifying a task by ID (`WHERE id = $1 AND user_id = $2`), unauthorized access returns `404 Not Found`. This prevents information leakage regarding whether a resource exists for another user.
* **Native `database/sql` + `pgx` Over GORM:** I opted for standard library `database/sql` with `pgx/v5` instead of an ORM like GORM to retain full control over query execution, eliminate runtime reflection overhead, and avoid ORM zero-value update pitfalls.
* **Separation of Connection Pooling from Schema (`pkg/database`):** I separated connection management (`postgres.go`) from DDL table creation (`schema.go`) to adhere to the Single Responsibility Principle.
* **Connection Pool Calibration:** I configured `SetMaxOpenConns(25)`, `SetMaxIdleConns(25)`, and `SetConnMaxLifetime(5 * time.Minute)` to prevent traffic surges from exhausting PostgreSQL connection limits while keeping idle connections warm.
* **Parameterized SQL Queries:** I used `$1, $2` bind parameters across all queries to ensure SQL injection defense and take advantage of prepared statements.
* **Resource Leak Defense (`defer rows.Close()`):** In `GetAll()`, I ensured `defer rows.Close()` is called immediately after `QueryContext()` to return connections to the pool upon query completion, followed by `rows.Err()` checking.
* **Context Propagation (`context.Context`):** I passed `request.Context()` down through the service layer to every database query so that if a client disconnects, PostgreSQL terminates the query immediately.
* **Idempotent DDL Schema Migrations:** In `pkg/database/schema.go`, I wrapped column additions in a PostgreSQL PL/pgSQL block (`DO $$ BEGIN IF NOT EXISTS ... THEN ALTER TABLE ... END IF; END $$;`). This guarantees safe execution on app startup without failing if the column already exists.

### HTTP Transport and Validation
* **Two-Tier Validation Strategy:** I placed format validation in the handler using DTO struct tags (`validate:"required,email"`, `validate:"required,min=8"`) so invalid requests are rejected with `422 Unprocessable Entity` before invoking the service or database layers. Domain rules remain in the service layer.
* **DTO Pointer Semantics for Partial Updates:** In `UpdateTaskDTO`, I used pointer types (`*string`) for optional fields. In Go, an unprovided primitive string field defaults to `""`. Pointers allow distinguishing between a field that was not sent (`nil`) and a field intentionally cleared (`""`).
* **Go 1.22+ Exact Path Matching (`/{$}`):** I used `"GET /{$}"` on the root route because a trailing slash in `http.ServeMux` acts as a wildcard catch-all. The `{$}` anchor ensures unknown routes properly return `404 Not Found`.
* **DRY Parameter Parsing:** I extracted `parseIDParam(request)` in the handler package to centralize `strconv.ParseInt`, ID positivity validation, and consistent error messaging across `GET`, `PUT`, and `DELETE` endpoints.
* **JSON Serialization Handling:** In `GetAllTasks()`, I explicitly return an empty slice `[]models.Task{}` instead of a `nil` slice when no records exist, ensuring the API returns `[]` instead of `null` in JSON responses.

---

## 3. Environment Configuration

The application loads environment variables from a `.env` file using `github.com/joho/godotenv` with sensible fallbacks:

| Variable | Description | Example Default |
| :--- | :--- | :--- |
| `PORT` | HTTP server listening port | `8080` |
| `APP_ENV` | Environment mode (`development` / `production`) | `development` |
| `POSTGRES_HOST` | PostgreSQL host address | `127.0.0.1` |
| `POSTGRES_PORT` | PostgreSQL port | `5432` |
| `POSTGRES_USER` | PostgreSQL user | `postgres` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `""` |
| `POSTGRES_DB` | Database name | `golang` |
| `POSTGRES_SSLMODE` | SSL connection mode | `disable` |
| `JWT_SECRET` | Base fallback secret | `your_secret_key` |
| `JWT_ACCESS_SECRET` | Secret used to sign short-lived access tokens | `your_access_secret` |
| `JWT_REFRESH_SECRET` | Secret used to sign long-lived refresh tokens | `your_refresh_secret` |

---

## 4. API Endpoints Reference

All endpoints are versioned under `/api/v1`:

### Public Endpoints
| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/health` | Health check verifying database connectivity | `200` |
| **POST** | `/api/v1/auth/register` | Register new user account and receive token pair | `201`, `400`, `409`, `422` |
| **POST** | `/api/v1/auth/login` | Authenticate user credentials and receive token pair | `200`, `400`, `401`, `422` |
| **POST** | `/api/v1/auth/refresh` | Exchange valid refresh token for a new token pair | `200`, `400`, `401`, `422` |
| **POST** | `/api/v1/auth/logout` | Invalidate active access token and/or refresh token | `200`, `400`, `500` |
| **GET** | `/` | Exact root match via `/{$}` | `200` |

### Protected Endpoints (Requires `Authorization: Bearer <access_token>`)
| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **POST** | `/api/v1/tasks` | Create task linked to authenticated user (defaults status to `pending`) | `201`, `400`, `401`, `422` |
| **GET** | `/api/v1/tasks` | List all tasks owned by authenticated user ordered by `created_at DESC` | `200`, `401` |
| **GET** | `/api/v1/tasks/{id}` | Fetch a task owned by authenticated user | `200`, `400`, `401`, `404` |
| **PUT** | `/api/v1/tasks/{id}` | Update task owned by authenticated user | `200`, `400`, `401`, `404`, `422` |
| **DELETE**| `/api/v1/tasks/{id}` | Remove a task owned by authenticated user | `200`, `400`, `401`, `404` |

---

## 5. Local Setup and Execution

1. **Clone the repository:**
   ```bash
   git clone https://github.com/getabalewKemaw/Golang.git
   cd Golang
   ```

2. **Configure environment:**
   ```bash
   cp .env.example .env
   # Update POSTGRES_* and JWT_* credentials in .env
   ```

3. **Run the application:**
   ```bash
   go run ./cmd/app/main.go
   ```
   *The database schema (tables `users`, `tasks`, and `revoked_tokens`) will be automatically created on startup.*
