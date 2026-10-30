# ezboot

**Distributed Go services with a lifecycle you can reason about.**

[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Release](https://img.shields.io/badge/release-v0.5.1-blue)](https://github.com/ez4bk/ezboot/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-green)](LICENSE)

English · [简体中文](README.zh-CN.md)

ezboot is a Go framework for building distributed services. Everything that makes up a process — the HTTP server, the gRPC server, MySQL, Redis, Kafka, OpenTelemetry — is a *plugin* implementing the same five methods. A small state machine starts them concurrently, waits until they are actually ready, and on shutdown drains and closes them in reverse order.

> [!WARNING]
> ezboot is pre-1.0 (`v0.5.x`): public APIs may still change between minor releases.

## Why ezboot

- **A lifecycle, not just a server.** Startup is a state machine with phases, budgets and rollback: a plugin that never becomes ready fails the boot instead of half-serving traffic, and `/readyz` flips to 503 before in-flight work is drained.
- **Explicit assembly.** There is no global container. Dependencies are wired at compile time with [wire](https://github.com/google/wire), and the shutdown order is the reverse of the list you wrote by hand — visible in one file instead of hidden in framework internals.
- **Batteries included, opt-in.** HTTP, gRPC with a gateway, six infrastructure plugins, tracing, RBAC and code generation ship in the box, but importing the framework does not open a Redis connection. You register the plugins you actually run.

The whole contract is five methods:

```go
type Plugin interface {
	Name() string
	Start(ctx context.Context) error // blocks until the plugin has stopped
	Ready(ctx context.Context) error // "can this instance take traffic?" — startup only
	Drain(ctx context.Context) error // stop accepting work, wait for in-flight work
	Close(ctx context.Context) error // release resources
}
```

Embed `PluginBase` to inherit no-op `Ready`/`Drain`/`Close`, then override only the phases you care about.

## Requirements

- Go 1.27 or newer
- Docker, only for the code-generation targets (`make protobuf`, `make wire`)

## Install

```bash
go get github.com/ez4bk/ezboot

# project generator
go install github.com/ez4bk/ezboot/cmd/ezboot@latest
```

## Quickstart

Scaffold a service from [ezboot-template](https://github.com/ez4bk/ezboot-template):

```bash
ezboot new demo
cd demo
```

The command asks for a Go module prefix first (for example `github.com/you`), then writes it into every import path and into `go.work`. The generated service enables the MySQL and Redis plugins, so start those first (see `config/local.yml` for the addresses and credentials it expects):

```bash
docker run -d --name demo-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=demo \
  -e MYSQL_USER=user -e MYSQL_PASSWORD=password mysql:8
docker run -d --name demo-redis -p 6379:6379 redis:7
```

Run it, then check readiness:

```bash
go run ./cmd/demo
curl -s localhost:8899/readyz   # {"status":"OK"}
```

`/readyz` answers 503 until every plugin's `Ready` has passed, and again the moment draining starts. `/_healthy` is liveness only — it is answered by the process, never by a plugin.

## Core concepts

| Concept | What it is |
|---|---|
| `Plugin` | A component with a lifecycle: `Start`, `Ready`, `Drain`, `Close`, `Name`. |
| `PluginBase` | Defaults for all five methods: `Start` blocks on the context, the rest are no-ops. |
| `State` | Two atomic flags (`started`, `draining`) written by the lifecycle and read by `/readyz`. Runtime readiness never probes plugins. |
| `Application` | `ezboot.NewApp(...)` plus options (`WithPlugins`, `WithState`, `WithLifecycleConfig`, `WithCmdline`). `Start(ctx)` returns only after drain and close have finished. |
| `ProviderSet` | The wire set for dependencies that every service needs and nobody customizes. |
| `Lifecycle` | The internal state machine that drives the plugins. Not exported; you observe it through `Application` and `State`. |

## Assembly with wire

ezboot assumes [google/wire](https://github.com/google/wire) as its dependency injection mechanism, and that assumption is part of the design rather than an implementation detail:

- **Compile-time graph.** Missing providers are build errors, not runtime panics. There is no service locator to fall back on.
- **Construction order is dependency order.** The tracer provider must exist before a gRPC server is built (interceptors are read when the driver is constructed), so wiring it as a constructor argument is what actually enforces the order.
- **Nothing is auto-registered.** A plugin runs because you put it in the list — not because a blank import did something.

`ezboot.ProviderSet` covers only what is universal and non-negotiable:

```go
var ProviderSet = wire.NewSet(
	ProvideConfig,        // config + telemetry init
	ProvideLogger,
	NewState,
	tracing.NewOTEL,
	grpcclient.NewGRPCClient,
)
```

HTTP, gRPC and the six infrastructure plugins are deliberately excluded. They need application-specific inputs (routes, service registrations, gateway handlers), and wire has no same-type multi-binding: putting `grpc.New` in the shared set collides with the application's own constructor.

One function must close `[]ezboot.Plugin`, and its argument order is the shutdown order:

```go
func NewPlugins(
	otel *tracing.OTEL,
	db *infra.Databases,
	gc *grpcclient.GRPCClient,
	gs *grpc.Server,
	hs *http.Server,
) []ezboot.Plugin {
	// Closed in reverse: http → grpc → grpc-client → db → otel
	return []ezboot.Plugin{otel, db, gc, gs, hs}
}
```

Adding a plugin therefore means changing this signature and regenerating (`wire ./...`; the template wraps it in `make wire`). That friction is intentional: it keeps the composition root reviewable in a diff.

## Lifecycle

```
Start (concurrent)
  → poll Ready          until all pass, or ready_timeout expires
  → started
  → wait for signal
  → draining            /readyz starts answering 503
  → Drain (reverse, serial)   stop accepting work, wait for in-flight work
  → cancel Start ctx
  → Close (reverse, serial)   release resources
```

- **Start runs concurrently**, so boot time is the slowest plugin, not the sum. A `Start` that returns before shutdown is a crash and tears the application down.
- **Ready is polled**, every 100 ms with each attempt capped at 2 s, until all plugins pass or `ready_timeout` expires. A failed boot rolls back: the remaining plugins are cancelled and everything already started is closed.
- **Drain then Close**, each in reverse registration order, each with its own fresh timeout context. They are not interchangeable: `Drain` stops accepting work and waits for in-flight work; `Close` releases resources.
- **Order matters.** Registration order *is* shutdown order: infrastructure first (closed last), entry points last (closed first), and OpenTelemetry at the very top so it flushes spans after everything else has stopped.

Budgets are configured under `app.lifecycle`; zero values fall back to the defaults shown:

```yaml
app:
  lifecycle:
    ready_timeout: 10s   # per plugin, startup
    drain_timeout: 10s   # per plugin
    close_timeout: 5s    # per plugin
```

In Kubernetes, keep `terminationGracePeriodSeconds` larger than the sum of the drain and close budgets, plus margin.

## Plugins

| Plugin | Package | Responsibility | `Ready` means |
|---|---|---|---|
| HTTP | `http` | gin engine, middleware, pprof, optional `/metrics`, `/_healthy`, `/readyz` | listener is bound |
| gRPC | `grpc` | server, gateway handler, reflection, Prometheus, graceful stop | listener bound, gateway ready |
| gRPC client pool | `grpc/client` | process-wide client connections, created on first use | always (connections are lazy) |
| OpenTelemetry | `tracing` | tracer provider, interception, span flush on close | always |
| Databases | `infra` | MySQL, SQLite or PostgreSQL through xorm — exactly one dialect per process | all configured pools ping |
| Redis | `infra` | clients per named instance | ping |
| Kafka | `infra` | brokers, readers and writers per topic | broker reachable |
| RabbitMQ | `infra` | connection with a reconnect loop | channel available |
| Elasticsearch | `infra` | search client | ping |
| Etcd | `infra` | client v3 | endpoint reachable |

Each infrastructure plugin ships its own `ProviderSet` with a default provider, so business code depends on the native type (`*xorm.Engine`, `*redis.Client`) rather than on a framework abstraction.

Writing your own plugin is the same five methods: embed `PluginBase`, implement `Name` and the phases you need, and add it to `NewPlugins`. Constructors may create client objects but must not perform network I/O or bind ports — that belongs in `Start`, and `Ready` is where a dependency is proven usable. The template includes a complete example under `internal/plugin/example`.

## Batteries

| Package | Purpose |
|---|---|
| `rbac` | Casbin-based RBAC model, policy definitions and loading |
| `exorm` | xorm query helpers: conditions, pagination, cursor pagination |
| `bizerr` | Business error type with gRPC status and gateway mapping |
| `snowflake` | Snowflake ID generation |
| `retry` | Exponential and constant retry built on backoff |
| `cache` | Redis-backed cache for binary-marshalable values |
| `flight` | Generic singleflight duplicate suppression |
| `exp` | Generic helpers: lazy values, maps, extraction, AES/RSA |
| `xjson` | JSON helpers |
| `xref` | Reflection helpers |
| `elog` | Log-and-wrap error helpers |
| `utils` | Pointer and string utilities |
| `signal` | `SIGINT`/`SIGTERM` context via `signal.WithExit` |
| `with` | Access to the default engine and Redis handle registered by plugins |

## Tooling

| Tool | Purpose |
|---|---|
| `ezboot new` | Scaffold a service from the template repository |
| `ezboot model` | Generate model definitions from an existing database schema |
| `protoc-gen-go-ezboot` | Protobuf generator for ezboot service glue |
| `protoc-gen-go-rbac` | Protobuf generator for RBAC definitions |
| `make protobuf` | Run `buf generate` in the pinned container image |
| `tools/reflect` | DAO generation from model structs |
| `cmd/snowflake` | Standalone Snowflake ID generator |

## Documentation

- [Wiki](https://github.com/ez4bk/ezboot/wiki) — technical details and architecture decision records (Chinese)
- [ezboot-template](https://github.com/ez4bk/ezboot-template) — reference service: wiring, a custom plugin, Helm chart
