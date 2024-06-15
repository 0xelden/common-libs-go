# common-libs-go

[![CI](https://github.com/0xelden/common-libs-go/actions/workflows/ci.yml/badge.svg)](https://github.com/0xelden/common-libs-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/0xelden/common-libs-go.svg)](https://pkg.go.dev/github.com/0xelden/common-libs-go)
![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)

A batteries-included Go foundation library for building microservices. It provides the framework layer a service is built on — structured errors, HTTP/gRPC plumbing, a generic repository layer, service discovery, object storage — plus code generators that scaffold repositories straight from SQL DDL.

## Features

- **Structured errors** (`serror`) — errors that carry stack traces, keys/codes, and comment stacks; every layer of the stack returns `serror.SError` instead of a bare `error`
- **HTTP layer** (`api`, `middleware`) — [gin](https://github.com/gin-gonic/gin)-based response envelope, list/detail query-param parsing (`IndexParam`, `ViewParam`, `SortParam`), JWT auth middleware, and OpenTelemetry tracing built in
- **Repository pattern** (`service`, `db`) — generic `Repository[dto, model]` contracts on top of [squirrel](https://github.com/Masterminds/squirrel) query building, with single- and multi-database transaction drivers; request-scoped transactions are auto-committed/rolled back by the response helpers
- **Service gateway** (`gateway`) — a Redis-backed service registry with HTTP→gRPC proxying and route/ACL handshakes between the gateway and its services
- **Object storage** (`objstorage`) — one interface, two backends: MinIO (default) or Azure Blob Storage, switched by env var
- **Connections** (`connection`) — env-driven constructors for PostgreSQL, MySQL, ClickHouse, and Redis
- **Code generation** (`cmd/gen-repo`) — generate repositories, controller scaffolds, and Postman collections from a SQL DDL file, non-destructively
- **Utilities** (`helper`, `types`) — generics helpers, datetime/SQL/string utilities, and JSON/SQL-serializable custom types (`JSONArray`, `NativeDate`, `RupiahCent`, …)

## Installation

```sh
go get github.com/0xelden/common-libs-go
```

Requires Go 1.25+.

## Quick start

### Structured errors

```go
import "github.com/0xelden/common-libs-go/serror"

func LoadConfig(path string) serror.SError {
	raw, err := os.ReadFile(path)
	if err != nil {
		serr := serror.NewFromError(err)
		serr.AddCommentf("while reading config %s", path)
		return serr
	}
	// ...
	return nil
}
```

`SError` captures the stack at creation; `Print()`/`ColoredString()` render it with file/line context, and comments accumulate as the error travels up the stack.

### HTTP handlers

```go
import (
	"github.com/gin-gonic/gin"
	"github.com/0xelden/common-libs-go/api"
)

func GetUser(c *gin.Context) {
	user, serr := userRepo.Get(c.Request.Context(), api.NewViewParam(c))
	if serr != nil {
		api.Error(c, serr)
		return
	}
	api.Success(c, user) // commits the request transaction, wraps in the response envelope
}
```

### Repositories

Repository implementations are generated (see below) and satisfy the generic contracts in `service`:

```go
type Repository[dto, model any] interface {
	Insert(ctx context.Context, form dto) (dto, serror.SError)
	Update(ctx context.Context, form dto, where ...sq.Sqlizer) (int64, serror.SError)
	Get(ctx context.Context, param api.ViewParam) (dto, serror.SError)
	List(ctx context.Context, param api.IndexParam) (api.RowIndex[dto], serror.SError)
	// ...
}
```

## Code generation

Generate a repository (and optionally a controller scaffold and Postman collection) from SQL DDL:

```sh
task gen-repo -- --ddl schema.sql --tables myschema.my_table --out . --ctl
task gen-repo:dry -- --ddl schema.sql --tables myschema.my_table   # preview only
task gen-columns                                                   # regenerate column constants from Dao structs
```

Generation is **non-destructive**: hand-written code outside the generated markers is preserved on regeneration (`--force` overwrites everything).

## Configuration

All configuration comes from environment variables, loaded from `.env` via [godotenv](https://github.com/joho/godotenv). Copy the template and fill it in:

```sh
cp .env.example .env
```

Key groups (see [.env.example](.env.example) for the full list):

| Group | Variables |
|---|---|
| App | `APP_ENV`, `APP_TIMEZONE`, `APP_HTTP_PORT`, `APP_GRPC_PORT`, … |
| Auth | `JWT_SECRET` |
| Registry / Redis | `APP_REGISTRY_ADDR`, `REDIS_HOST` |
| Database | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PWD`, `DB_NAME` |
| Object storage | `STORAGE_DRIVER` (`minio`/`azure`), `STORAGE_MINIO_*`, `STORAGE_AZURE_*` |
| Tracing | `UPTRACE_DSN` |

## Development

Uses [Task](https://taskfile.dev) as the task runner (POSIX shell required; use Git Bash on Windows).

```sh
task            # tidy + fmt + vet + test, then build
task test       # run the test suite
task vet        # static analysis
task nilaway    # nilness analysis (optional, requires nilaway)
task docs:serve # serve mdbook docs on :3321 (requires gomarkdoc + mdbook)
```

Integration tests self-skip when their backing services aren't configured (`MINIO_HOST`, `DB_NAME`, …), so the suite runs green without any infrastructure.

## Contributing

1. Fork and create a feature branch
2. Make your changes; keep `task` (validate + build) passing
3. Open a pull request against the `dev` branch (`task pr` automates this via the [GitHub CLI](https://cli.github.com/))

Changes flow `dev` → `qa` → `main`. CI runs tidy-check, vet, tests, and build on every PR.
