# Go Backend Architecture and Technical Concepts Report

This document outlines the architectural decisions, Go runtime mechanics, database management strategies, and engineering principles implemented in this backend project.

---

## 1. Architectural Design and Component Boundaries

The project implements a decoupled, layered Clean Architecture to enforce the Single Responsibility Principle (SRP) and ensure independent testability across layers.

```text
HTTP Request
     │
     ▼
[ routes ]        URL mapping, path matching, API versioning (/api/v1)
     │
     ▼
[ handlers ]      HTTP transport: JSON decoding, DTO validation, status codes
     │
     ▼
[ services ]      Business logic: domain rules, defaults, data transformation
     │
     ▼
[ repositories ]  Persistence: parameterized SQL execution, row scanning
     │
     ▼
PostgreSQL DB     Connection pool, tables, constraints
```

### Dependency Injection Pattern in Go
Go does not require reflection-heavy IoC containers or frameworks (like Spring or NestJS). Dependencies are passed explicitly through constructors:
1. `pkg/database` initializes the database pool (`*sql.DB`).
2. `repositories.NewPostgresTaskRepository(databasePool)` receives the pool.
3. `services.NewTaskService(taskRepository)` receives the `TaskRepository` interface.
4. `handlers.NewTaskHandler(taskService)` receives the `TaskService` interface.
5. `routes.SetupRouter(taskHandler, databasePool)` wires endpoints to the router.

### Why Code to Interfaces?
The Service layer does not depend on `PostgresTaskRepository` directly; it depends on the `TaskRepository` interface. 
* In Go, interfaces are implemented **implicitly** (no `implements` keyword).
* This allows the Service layer to be unit-tested using mock repositories without connecting to a real database.

---

## 2. Go Runtime Mechanics and Language Fundamentals

### Pointers vs. Values (`*` and `&`)
* **Default Pass-by-Value:** In Go, passing a struct to a function creates a shallow copy of that struct in memory.
* **Why Pointers are Used:**
  * For large structs (e.g., `*models.Task`, `*http.Request`), passing a pointer (`8 bytes`) avoids duplicating data in memory.
  * Pointers allow functions to mutate the underlying struct directly in memory (e.g., `repo.Create(ctx, task)` writes the generated database ID directly into `task.ID`).
* **Structs vs. Interfaces:**
  * `r *http.Request` has a pointer (`*`) because `http.Request` is a concrete data struct.
  * `w http.ResponseWriter` does not have a pointer (`*`) because it is an **interface**. In Go, interface values already wrap an internal pointer to the underlying implementation.

### The `defer` Lifecycle
* `defer` schedules a function call to execute immediately before the surrounding function returns.
* It does not run when defined; it runs at the exit door of the function.
* **Critical uses in this project:**
  * `defer databasePool.Close()` in `main.go`: ensures the connection pool is closed on application shutdown.
  * `defer rows.Close()` in `GetAll()`: ensures database connections are returned to the pool once rows are read.
  * `defer cancel()`: releases context timers immediately to prevent memory leaks.

### Zero Values in Go
Go initializes variables to their "zero value" instead of `null` or `undefined`:
* `int` -> `0`
* `string` -> `""`
* `bool` -> `false`
* `time.Time` -> `0001-01-01 00:00:00 UTC` (not current time)
* **Design decision:** We use `time.Now()` or Postgres `DEFAULT NOW()` to ensure valid timestamps, and pointers (`*string`, `*TaskStatus`) in `UpdateTaskDTO` to differentiate between omitted fields (`nil`) and empty values (`""`).

### Type Assertions
In `pkg/validator`, the validation engine returns a generic `error`. Go's type assertion syntax `err.(validator.ValidationErrors)` safely inspects the concrete type behind the interface:
```go
validationErrors, ok := err.(validator.ValidationErrors)
if !ok {
    return map[string]string{"error": "invalid payload"}
}
```
The `ok` boolean prevents the application from panicking if the error is of a different concrete type.

### Error Handling Philosophy
Go avoids runtime exceptions (`try / catch / throw`) in favor of explicit error returns:
* Functions return `(result, error)` tuples.
* Errors are checked immediately: `if err != nil`.
* **Sentinel Errors:** Defined at package level (`ErrTaskNotFound`, `ErrInvalidTaskID`) and checked using `errors.Is(err, target)`.
* **Error Wrapping:** Using `fmt.Errorf("...: %w", err)` to preserve the root cause while adding layer-specific context.

---

## 3. Database Management and Resource Optimization

### Why `database/sql` + `pgx` Over Heavy ORMs (GORM)
For production systems, standard `database/sql` was chosen over ORMs like GORM:
1. **Performance:** Eliminates runtime reflection overhead.
2. **Predictability:** Raw SQL ensures exact control over query execution plans and index utilization.
3. **Avoids Zero-Value Gotchas:** Many ORMs fail to update zero values (e.g., setting a boolean to `false` or integer to `0`) without special update flags.

### Connection Pool Configuration
To prevent application traffic from crashing PostgreSQL, the connection pool is explicitly configured:
* `SetMaxOpenConns(25)`: Sets a ceiling on active TCP connections to stay within database limits.
* `SetMaxIdleConns(25)`: Keeps warm connections in memory to avoid the latency of TCP three-way handshakes on every request.
* `SetConnMaxLifetime(5 * time.Minute)`: Periodically recycles connections to prevent stale connections from being terminated by firewalls or cloud load balancers.

### Context and Query Cancellation (`context.Context`)
Every repository query receives `ctx context.Context` derived from `request.Context()`:
* If a client disconnects or aborts the HTTP request before the database query finishes, Go sends an interrupt signal to PostgreSQL to terminate the query.
* This prevents wasted CPU, RAM, and database lock contention on abandoned client requests.

### SQL Injection Prevention
All SQL queries use parameterized bind variables (`$1, $2, $3`):
* Queries are sent as prepared statements where SQL logic and user data travel separately.
* User input is treated strictly as literal data, making SQL injection impossible.

### Idempotent Schema Management
Table definitions are isolated in `pkg/database/schema.go`:
* Uses `CREATE TABLE IF NOT EXISTS tasks (...)`.
* Executed automatically on application boot within a 5-second timeout, ensuring the database is always prepared without manual migration scripts for local runs.

---

## 4. HTTP Transport, Routing, and Validation

### Go 1.22+ Native Routing Features
The project uses the standard library `http.ServeMux` with features introduced in Go 1.22:
1. **Method Matching:** Routes define their HTTP verbs directly (`POST /api/v1/tasks`, `GET /api/v1/tasks`).
2. **Path Parameter Parsing:** Path variables like `/tasks/{id}` are extracted natively using `request.PathValue("id")` without third-party routers like Gorilla Mux.
3. **Exact Path Matching (`/{$}`):** 
   * In Go, patterns ending with `/` act as subtree wildcards (matching all sub-paths).
   * Using `GET /{$}` restricts the root route strictly to `/`, allowing all unrecognized URLs (like `/ll///`) to correctly return `404 Not Found`.

### Two-Tier Validation Strategy
1. **Transport / Format Validation (Handler Layer):**
   * Uses DTO struct tags (`validate:"required,min=1,max=100"`).
   * Validated as soon as JSON is decoded in the handler.
   * Rejects malformed requests with `422 Unprocessable Entity` before invoking business services.
2. **Business / Semantic Validation (Service Layer):**
   * Validates business logic (e.g., verifying `taskID > 0`, verifying existence before update, applying default statuses).

### DRY Handler Patterns
Repeated logic (e.g., path parameter parsing, bounds checking, JSON response serialization) is consolidated into handler utility functions:
* `parseIDParam(request)`: centralizes `strconv.ParseInt`, parameter validation, and consistent error messaging.
* `writeJSON(responseWriter, statusCode, data)`: standardizes `Content-Type: application/json` headers and status code writing.

---

## 5. Summary of API Endpoints

All endpoints are versioned under `/api/v1`:

| Method | Endpoint | Description | Status Codes |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/health` | Service and database connectivity check | `200` |
| **POST** | `/api/v1/tasks` | Create task (defaults status to `pending`) | `201`, `400`, `422` |
| **GET** | `/api/v1/tasks` | Retrieve all tasks ordered by creation date | `200` |
| **GET** | `/api/v1/tasks/{id}` | Retrieve task by primary key ID | `200`, `400`, `404` |
| **PUT** | `/api/v1/tasks/{id}` | Update task fields (partial or full) | `200`, `400`, `404`, `422` |
| **DELETE**| `/api/v1/tasks/{id}` | Delete task by primary key ID | `200`, `400`, `404` |
| **GET** | `/` | Root endpoint (exact match via `/{$}`) | `200` |
