# Task Management Backend - Architecture & Engineering Decisions

This document summarizes the architectural decisions, design choices, and implementation details for this Go backend project.

---

## 1. Project Overview and Directory Layout

The project follows a modular, layered structure to enforce separation of concerns:

```text
d:\golang\
├── cmd/
│   └── app/
│       └── main.go                  # Application bootstrap and dependency wiring
├── configs/
│   └── config.go                # Strongly-typed environment configuration
├── internal/                    # Private domain and application logic
│   ├── dto/
│   │   └── task_dto.go          # Request DTOs with struct validation tags
│   ├── handlers/
│   │   └── task_handler.go      # HTTP controllers (JSON decoding, status codes)
│   ├── models/
│   │   └── task.go              # Pure domain database entity
│   ├── repositories/
│   │   └── task_repository.go  # SQL queries and repository interface
│   ├── routes/
│   │   ├── routes.go            # Router configuration and health checks
│   │   └── task_routes.go       # Route definitions under /api/v1/tasks
│   └── services/
│       └── task_service.go      # Business rules and service layer implementation
├── pkg/                         # Reusable infrastructure packages
│   ├── database/
│   │   ├── postgres.go          # Connection pool initialization and ping verification
│   │   └── schema.go            # Idempotent table creation (CREATE TABLE IF NOT EXISTS)
│   └── validator/
│       └── validator.go         # Struct tag validation wrapper
├── .env                         # Local environment settings
├── .env.example                 # Example environment template
├── .gitignore                   # Ignored files (binaries, env files)
├── go.mod                       # Module definitions
└── go.sum                       # Dependency checksums
```

---

## 2. Key Engineering Decisions and Rationale

### Architecture and Layering
* **Layered Clean Architecture:** I separated the code into `handlers`, `services`, and `repositories` so that HTTP logic, business rules, and SQL data access never leak into one another.
* **Constructor-Based Dependency Injection:** I avoided global state and reflection-based containers. Each component accepts its dependencies via a constructor (`New...`), passing dependencies from the database pool up to the router in `main.go`.
* **Interface-Driven Repositories:** I defined `TaskRepository` as an interface that `PostgresTaskRepository` implements. This decouples the service layer from PostgreSQL and allows for mock-based unit testing.
* **Pure Controllers (`internal/handlers`) vs. Routes (`internal/routes`):** I extracted route registration out of the handlers and into a dedicated `routes` package. Handlers act strictly as controllers, while URL paths and `/api/v1` versioning are managed centrally.

### Database and Persistence
* **Native `database/sql` + `pgx` Over GORM:** I opted for standard library `database/sql` with `pgx/v5` instead of an ORM like GORM to retain full control over query execution, eliminate runtime reflection overhead, and avoid ORM zero-value update pitfalls.
* **Separation of Connection Pooling from Schema (`pkg/database`):** I separated connection management (`postgres.go`) from DDL table creation (`schema.go`) to adhere to the Single Responsibility Principle.
* **Connection Pool Calibration:** I configured `SetMaxOpenConns(25)`, `SetMaxIdleConns(25)`, and `SetConnMaxLifetime(5 * time.Minute)` to prevent traffic surges from exhausting PostgreSQL connection limits while keeping idle connections warm.
* **Parameterized SQL Queries:** I used `$1, $2` bind parameters across all queries to ensure SQL injection defense and take advantage of prepared statements.
* **Resource Leak Defense (`defer rows.Close()`):** In `GetAll()`, I ensured `defer rows.Close()` is called immediately after `QueryContext()` to return connections to the pool upon query completion, followed by `rows.Err()` checking.
* **Context Propagation (`context.Context`):** I passed `request.Context()` down through the service layer to every database query so that if a client disconnects, PostgreSQL terminates the query immediately.

### HTTP Transport and Validation
* **Two-Tier Validation Strategy:** I placed format validation in the handler using DTO struct tags (`validate:"required,min=1,max=100"`) so invalid requests are rejected with `422 Unprocessable Entity` before invoking the service or database layers. Domain rules (e.g., status defaults and bounds checks) remain in the service layer.
* **Go 1.22+ Exact Path Matching (`/{$}`):** I used `"GET /{$}"` on the root route because a trailing slash in `http.ServeMux` acts as a wildcard catch-all. The `{$}` anchor ensures unknown routes properly return `404 Not Found`.
* **DRY Parameter Parsing:** I extracted `parseIDParam(request)` in the handler package to centralize `strconv.ParseInt`, ID positivity validation, and consistent error messaging across `GET`, `PUT`, and `DELETE` endpoints.
* **JSON Serialization Handling:** In `GetAllTasks()`, I explicitly return an empty slice `[]models.Task{}` instead of a `nil` slice when no records exist, ensuring the API returns `[]` instead of `null` in JSON responses.

---

## 3. API Endpoints Reference

All endpoints are versioned under `/api/v1`:

| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/health` | Health check verifying database connectivity | `200` |
| **POST** | `/api/v1/tasks` | Create task (defaults status to `pending` if omitted) | `201`, `400`, `422` |
| **GET** | `/api/v1/tasks` | List all tasks ordered by `created_at DESC` | `200` |
| **GET** | `/api/v1/tasks/{id}` | Fetch a task by primary key | `200`, `400`, `404` |
| **PUT** | `/api/v1/tasks/{id}` | Update task fields (partial or full) | `200`, `400`, `404`, `422` |
| **DELETE**| `/api/v1/tasks/{id}` | Remove a task by primary key | `200`, `400`, `404` |
| **GET** | `/` | Exact root match via `/{$}` | `200` |
