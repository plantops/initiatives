# Architecture v0.1

## Principle

One Go process, one relational database, one attachment store.

```text
HTTP
 |
 |-- auth/session/RBAC
 |-- initiative handlers
 |-- master-data handlers
 |
 +--> store.go --> database/sql --> SQLite
 |
 +--> attachment directory
```

No service split. No message queue. No SPA. No ORM.

## SST

Authoritative data:

```text
initiative DB + attachment metadata
```

Generated Excel/PDF/report files are outputs only.

## Master data boundary

Business master:
- department
- employee
- problem category
- benefit type
- unit
- value period
- implementation status
- document type
- reward level

Application-controlled master:
- workflow status
- RBAC role
- review decision

## Portability rule

SQLite-specific DDL is embedded in `internal/app/migrations/001_init.sql`.
Application SQL is kept in `store.go`/handlers and uses ordinary relational columns.
When PostgreSQL is introduced, create a separate PostgreSQL migration and driver rather than making one schema file conditional.
