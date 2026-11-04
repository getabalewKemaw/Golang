# Task Management Backend API

A RESTful API built in Go (Golang) and PostgreSQL following layered architecture, dependency injection, and standard library conventions.

---

## Architecture and Design Pattern

The application enforces separation of concerns across distinct layers:

```text
HTTP Client (Postman / Browser)
         │
         ▼
[ internal/routes ]        URL routing and API versioning (/api/v1)
         │
         ▼
[ internal/handlers ]      Controllers: parse JSON, run DTO validation, return HTTP responses
         │
         ▼
[ internal/services ]      Business Logic: domain rules, defaults, data transformation
         │
         ▼
[ internal/repositories ]  Data Access: parameterized SQL queries, row scanning
         │
         ▼
   PostgreSQL DB           Connection pooling, tables, indexes
```

### Dependency Injection
Dependencies are injected explicitly via constructors from the database up to the router:
1. `pkg/database` connects to Postgres and returns a `*sql.DB` connection pool.
2. `*sql.DB` is injected into `repositories.NewPostgresTaskRepository()`.
3. `TaskRepository` interface is injected into `services.NewTaskService()`.
4. `TaskService` interface is injected into `handlers.NewTaskHandler()`.
5. `TaskHandler` is passed into `routes.SetupRouter()`.

---

## Project Structure

```text
d:\golang\
├── cmd/
│   └── app/
│       └── main.go                  # Application entry point and dependency wiring
├── configs/
│   └── config.go                # Strongly-typed environment configuration
├── internal/                    # Private application code
│   ├── dto/
│   │   └── task_dto.go          # Request payloads and validation tags
│   ├── handlers/
│   │   └── task_handler.go      # HTTP controllers and request helpers
│   ├── models/
│   │   └── task.go              # Task domain entity
│   ├── repositories/
│   │   └── task_repository.go  # SQL queries and TaskRepository interface
│   ├── routes/
│   │   ├── routes.go            # Root router and health check routes
│   │   └── task_routes.go       # Route definitions for /api/v1/tasks
│   └── services/
│       └── task_service.go      # Business logic implementation
├── pkg/                         # Reusable packages (no domain logic)
│   ├── database/
│   │   ├── postgres.go          # Connection pool configuration and ping check
│   │   └── schema.go            # Table creation (CREATE TABLE IF NOT EXISTS)
│   └── validator/
│       └── validator.go         # Struct tag validation wrapper
├── .env                         # Local environment variables
├── .env.example                 # Example environment template
├── .gitignore                   # Git ignore patterns
├── go.mod                       # Go module dependencies
└── go.sum                       # Dependency checksums
```

---

## Core Technical Concepts Implemented

1. **Standard Library database/sql:**
   * Uses parameterized queries (`$1, $2`) for SQL injection protection.
   * `rows.Scan()` maps database columns directly into Go struct fields.
   * `defer rows.Close()` ensures database connections are returned to the pool after query completion.
2. **Connection Pooling:**
   * Configured `SetMaxOpenConns(25)`, `SetMaxIdleConns(25)`, and `SetConnMaxLifetime(5 * time.Minute)`.
3. **Context Cancellation:**
   * Passes `request.Context()` to all database operations so cancelled requests immediately terminate ongoing database queries.
4. **DTO Validation:**
   * Uses struct tags (`validate:"required,min=1,max=100"`) to validate inputs in the handler before calling the service layer.
5. **Sentinel Errors:**
   * Defines domain-level errors (such as `ErrTaskNotFound`) so handlers can return appropriate HTTP status codes (e.g., 404).

---

## Setup and Execution

### 1. Prerequisites
* Go 1.22 or higher
* PostgreSQL running on port 5432

### 2. Environment Configuration
Create a `.env` file in the project root:

```env
PORT=8080
APP_ENV=development

POSTGRES_HOST=127.0.0.1
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=postgres
POSTGRES_SSLMODE=disable
```

### 3. Run the Application
```powershell
go run .\cmd\app\main.go
```

Startup output:
```text
Connected to PostgreSQL successfully!
Database table 'tasks' verified/created successfully!
Server starting in [development] mode on http://localhost:8080
API Base URL: http://localhost:8080/api/v1
Health check: http://localhost:8080/api/v1/health
```

---

## API Endpoints (Postman Reference)

Base URL: `http://localhost:8080`

### 1. Health Check
* **Method:** `GET`
* **URL:** `http://localhost:8080/api/v1/health`
* **Status:** `200 OK`
```json
{
  "status": "ok",
  "database": "connected"
}
```

---

### 2. Create Task
* **Method:** `POST`
* **URL:** `http://localhost:8080/api/v1/tasks`
* **Headers:** `Content-Type: application/json`
* **Request Body:**
```json
{
  "title": "Complete clean architecture",
  "description": "Implement handlers, services, and repositories",
  "status": "in_progress"
}
```
* **Success Status:** `201 Created`
```json
{
  "id": 1,
  "title": "Complete clean architecture",
  "description": "Implement handlers, services, and repositories",
  "status": "in_progress",
  "created_at": "2026-11-04T16:28:31.123456Z",
  "updated_at": "2026-11-04T16:28:31.123456Z"
}
```
* **Validation Failure:** `422 Unprocessable Entity`
```json
{
  "errors": {
    "title": "title is required"
  }
}
```

---

### 3. Get All Tasks
* **Method:** `GET`
* **URL:** `http://localhost:8080/api/v1/tasks`
* **Status:** `200 OK`
```json
[
  {
    "id": 1,
    "title": "Complete clean architecture",
    "description": "Implement handlers, services, and repositories",
    "status": "in_progress",
    "created_at": "2026-11-04T16:28:31.123456Z",
    "updated_at": "2026-11-04T16:28:31.123456Z"
  }
]
```

---

### 4. Get Task by ID
* **Method:** `GET`
* **URL:** `http://localhost:8080/api/v1/tasks/1`
* **Success Status:** `200 OK`
```json
{
  "id": 1,
  "title": "Complete clean architecture",
  "description": "Implement handlers, services, and repositories",
  "status": "in_progress",
  "created_at": "2026-11-04T16:28:31.123456Z",
  "updated_at": "2026-11-04T16:28:31.123456Z"
}
```
* **Not Found Status:** `404 Not Found`
```json
{
  "error": "task not found"
}
```
* **Invalid ID Status:** `400 Bad Request`
```json
{
  "error": "invalid task id in path: must be a positive integer"
}
```

---

### 5. Update Task
* **Method:** `PUT`
* **URL:** `http://localhost:8080/api/v1/tasks/1`
* **Headers:** `Content-Type: application/json`
* **Request Body:**
```json
{
  "title": "Updated Task Title",
  "status": "completed"
}
```
* **Success Status:** `200 OK`
```json
{
  "id": 1,
  "title": "Updated Task Title",
  "description": "Implement handlers, services, and repositories",
  "status": "completed",
  "created_at": "2026-11-04T16:28:31.123456Z",
  "updated_at": "2026-11-04T16:45:10.987654Z"
}
```
* **Not Found Status:** `404 Not Found`

---

### 6. Delete Task
* **Method:** `DELETE`
* **URL:** `http://localhost:8080/api/v1/tasks/1`
* **Success Status:** `200 OK`
```json
{
  "message": "task deleted successfully"
}
```
* **Not Found Status:** `404 Not Found`
```json
{
  "error": "task not found"
}
```
