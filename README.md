# Initiative Workflow — Local MVP

Lean/KISS internal web app for the company initiative program.

Current deployment:

```text
Browser on Win10
    -> Go net/http
    -> server-rendered HTML
    -> database/sql
    -> SQLite file
    -> local attachment folder
```

No Node.js. No frontend build. No Docker required.

## Stack

- Go 1.25.1 or newer.
- `net/http` + `html/template` from the Go standard library.
- `database/sql`.
- `modernc.org/sqlite` v1.57.0: pure Go SQLite driver; no CGO requirement.
- SQLite database: `data/initiative.db`.
- Attachments: `data/uploads/`.

## Run on Windows 10

Go 1.25.1 already works. Open PowerShell:

```powershell
cd initiative-app
.\run.ps1
```

Or manually:

```powershell
go mod download
go run ./cmd/server
```

Open:

```text
http://127.0.0.1:8080
```

On first run `/setup` creates the first department, employee and `ADMIN` user. There is no default password.

Build a local executable:

```powershell
.\build-win.ps1
.\bin\initiative.exe
```

## RBAC

Roles are additive:

| Role | Scope |
|---|---|
| `EMPLOYEE` | Own initiatives |
| `MANAGER` | Own + initiatives in same department; manager review |
| `HRD` | All initiatives; HRD forwarding, verification, reward, close |
| `COUNCIL` | All initiatives; council assessment |
| `GM` | All initiatives; GM approval |
| `ADMIN` | All records + master data + user/RBAC administration |

RBAC is enforced in handlers, not only hidden in the UI.

## Workflow

```text
DRAFT
 -> MANAGER_REVIEW
 -> HRD_REVIEW
 -> COUNCIL_REVIEW
 -> GM_APPROVAL
 -> APPROVED
 -> VERIFICATION
 -> REWARDED
 -> CLOSED
```

Manager reject path:

```text
MANAGER_REVIEW -> MANAGER_REJECTED
```

Implementation status remains a separate dimension:

```text
IDEA | PILOT | IMPLEMENTED
```

## Canonical record

The main record uses controlled master-data codes for:

- proposer/department;
- problem category;
- benefit type;
- implementation status;
- workflow status;
- value period;
- document type;
- reward level;
- RBAC role.

Economic value is stored as numeric VND, with expected and actual values separated.

## Master data

Admin page:

```text
/admin/master
```

The MVP lets ADMIN maintain departments, employees and user/role assignments. System masters are seeded and visible there. Workflow/status code masters are intentionally not editable from the UI.

## Backups

For this local MVP, back up:

```text
data/initiative.db
data/uploads/
```

Stop the app before taking a simple file copy. For later online deployment use database-native backups and object storage backups.

## Public/PostgreSQL scale path

Keep the Go application and replace infrastructure in this order:

```text
Phase 0  Win10 + SQLite + local files
Phase 1  Linux host + Go binary + PostgreSQL + filesystem/object storage
Phase 2  Public URL + reverse proxy/TLS + PostgreSQL backups + object storage
Phase 3  SSO/AD, audit hardening, notifications, reporting if justified
```

Do not redesign the application before load or business requirements justify it.

The current SQL access is isolated under `internal/app/store.go`. Business identity is `initiative_code`, not the SQLite row id. SQL avoids JSON blobs. A PostgreSQL migration still requires a PostgreSQL schema/driver and query compatibility review; it is not a zero-change driver swap.

Before exposing this MVP publicly, add the controls listed in `docs/PUBLIC_DEPLOYMENT_GATE.md`.
