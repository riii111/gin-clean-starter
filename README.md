# Gin Clean Starter

🏗️ **Clean Architecture + CQRS + DDD + UoW (using Golang/Gin)** — Production-ready booking system template with **idempotent APIs** & **race-safe reservations**

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-blue" />
  <img src="https://img.shields.io/badge/DB-PostgreSQL + tstzrange-green" />
  <img src="https://img.shields.io/badge/DI-Uber FX-orange" />
  <img src="https://img.shields.io/badge/ORM-sqlc-red" />
</p>

---

## 🎯 What's Inside

|  | Feature | Why It Matters |
|---|---|---|
| 🏛️ **Clean Architecture** | Domain/UseCase/Infra layers | Easy testing & maintenance |
| ⚡ **Race-Safe Reservations** | DB-level conflict prevention | No double-bookings ever |  
| 🔄 **True Idempotency** | Request deduplication + result caching | API clients can retry safely |
| 🎫 **Flexible Coupons** | Fixed amount or percentage discounts | Business requirement ready |
| 🔐 **JWT + RBAC** | Role-based access (viewer/operator/admin) | Production auth patterns |

---

## 🚀 Quick Start

```bash
git clone <this-repo>
cd gin-clean-starter
mise install && mise run install       # Install tools & dependencies
mise run up                            # Start PostgreSQL
mise run migrate:up                    # Apply schema
```

---

## 🗄️ Database Operations

### Migrations (Atlas-powered)
```bash
mise run migrate:up        # Apply migrations
mise run migrate:status    # Check status
mise run migrate:down      # Rollback
```

### After editing migration files
```bash
mise run migrate:hash      # Fix checksum errors
mise run migrate:up        # Then apply
```

### Code generation
```bash
mise run sqlc:gen          # Regenerate type-safe DB code
```

---

## 🛠️ Development Commands

| Task | Command | Description |
|------|---------|-------------|
| **Test** | `mise run test-all` | Unit + E2E tests |
| **Lint** | `mise run lint:fix` | Auto-fix code issues |
| **Build** | `mise run build` | Docker images |

<details>
<summary>All available commands</summary>

```bash
# Environment
mise run up              # Start services
mise run down            # Stop services  
mise run logs            # View logs

# Code quality
mise run lint            # Check issues
mise run fmt             # Format code
mise run sql:format      # Format SQL

# Testing
mise run test-unit       # Unit tests only
mise run test-e2e        # E2E tests only
mise run test-clean      # Clean test cache
```

</details>

---

## 📡 API Highlights

All endpoints require auth (except `/auth/login`). Uses `Idempotency-Key` header for safe retries.

```bash
# Login
curl -X POST localhost:8888/auth/login \
  -d '{"email":"test@example.com","password":"password123"}'

# Create reservation (idempotent)
curl -X POST localhost:8888/reservations \
  -H "Authorization: Bearer <token>" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"resource_id":"...","start_time":"2024-01-01T10:00:00Z",...}'
```

Swagger docs: `http://localhost:8888/swagger/` (debug mode)

### API Conventions
- Cursor format: keyset pagination uses Base64URL cursor `v1:<created_at_unix_micro>-<uuid>` encoded as Base64URL. Invalid cursor → 400.
- Errors: map infrastructure/usecase errors to HTTP codes consistently — 400 (invalid input), 401 (unauthorized), 403 (forbidden), 404 (not found), 409 (conflict), 500 (internal error).

---

## 🏗️ Architecture

```
internal/
├── domain/          # Pure business logic (no dependencies)
├── usecase/         # Application services + interfaces  
├── handler/         # HTTP layer (Gin routes + middleware)
├── infra/          # Database, external services
└── pkg/            # Shared utilities
```

**Key Patterns:**
* **CQRS** — Separate read/write models for performance  
* **Repository** — Clean data access abstraction
* **DI** — Uber FX for dependency injection  
* **Value Objects** — Type-safe domain primitives

---

## 🗂️ Project Context

Built for **booking/reservation systems** where race conditions and data consistency are critical. Includes real-world patterns like:

* **Time slot conflicts** → Prevented at DB level with EXCLUDE constraints
* **Request duplication** → Handled with proper idempotency (not just dedup)  
* **Domain validation** → Clean separation from HTTP concerns
* **Role-based access** → JWT with viewer/operator/admin levels

Check `.docs/` folder for detailed requirements and API specifications.
