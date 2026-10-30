# ezboot plugin architecture and lifecycle design

> Status: Draft (2026-10-28)
> Scope: `internal/gin-boot/boot.go`, `app.go`, `http/`, `grpc/`, `internal/gin-boot/infra/`, wire and the template assembly.
> This document describes the design and the migration path only, not the implementation.

> English translation of [the Chinese original](plugin-lifecycle.md). Package names, historical references and the migration record are kept verbatim, including paths that no longer exist after the migration completed.

## 1. Background

The framework had three intertwined problems:

1. **A closed composition root.** The startup flow was hard-coded in `boot.Default()` and `boot.Server.Run()`, so adding a component (a websocket hub, a consumer, an admin server) required changing framework internals.
2. **I/O during construction.** `http.New` called `boot.Default()`, which triggered config → migration → `infra.Open` → OTEL → gRPC client in one go. Components were effectively views over global singletons, and two applications could not be constructed in one process.
3. **Two startup paths, and the live one was incomplete.** `Server.Run()` (`internal/gin-boot/boot.go:315`) contained pprof, grpc_prometheus, background handlers and `OnShutdown` (closing gRPC clients and infra) — but it had **no callers** in the repository. The real path was `Application.serve` (`app.go:34`), which only started HTTP and gRPC concurrently: no Close, no drain timeout, no infra release.

The goal of this design is therefore not "make the components look alike", but to **move the composition root out of the framework and into the caller**, and to turn startup and shutdown into a state machine with explicit phases, explicit timeouts and rollback.

## 2. Goals / non-goals

**Goals**

- Components — official and user-written — implement one interface and are selected explicitly by the caller in wire.
- Startup and shutdown have defined phases, timeouts and error propagation; failures can be rolled back.
- Official plugins and user plugins take exactly the same assembly path (official plugins get no privileges).
- Delete dead code and remove `boot.Default()` from the role of composition root.

**Non-goals**

- Runtime plugin loading (the `plugin` package, a separate process, RPC extensions). The cost is too high under Go's cross-compilation and container-image constraints; explicitly out of scope.
- Multiple instances in one process. This design removes the `infra` global, but `viper`, `config.Conf`, `gin.SetMode` and the default logging instance are still package-level state; multi-instance support is out of scope here.
- Runtime dependency probing (see the decision log in §10).

## 3. Core concepts

| Concept | Description |
|---|---|
| Plugin | A component that participates in the application lifecycle, with 5 methods |
| PluginBase | No-op implementations of 4 phases; after embedding, only `Name` is required |
| State | Two atomic booleans, `started` / `draining`; the single source of truth for `/readyz` |
| Lifecycle | The phase machine implementation, an internal package, not exported |

## 4. Interface

```go
// Plugin is a pluggable component in the application lifecycle.
// Phase order: Start → poll Ready → Drain → Close.
type Plugin interface {
    Name() string
    Start(ctx context.Context) error
    Ready(ctx context.Context) error
    Drain(ctx context.Context) error
    Close(ctx context.Context) error
}

// PluginBase provides default implementations for the four phases. After
// embedding it you only need to implement Name, plus the phases you care about.
type PluginBase struct{}

// Start blocks until ctx is done. Start returning means the plugin has stopped,
// and lifecycle relies on that signal to tell whether the plugin is still alive.
func (PluginBase) Start(ctx context.Context) error { <-ctx.Done(); return nil }

// Ready assumes the plugin is available at any time. Plugins with connection
// semantics must override it.
func (PluginBase) Ready(context.Context) error { return nil }

func (PluginBase) Drain(context.Context) error { return nil }
func (PluginBase) Close(context.Context) error { return nil }
```

### 4.1 Contract

`Start`

- Blocks until the plugin stops. **Overriding `Start` means writing `<-ctx.Done()` yourself**; otherwise it is treated as "exited right after starting".
- Returning `nil` means a normal stop; returning an error means a failure.
- Constructors may **create client objects** (`redis.NewClient`, `xorm.NewEngine`, `elasticsearch.NewClient`), but must not perform network I/O or bind ports. Connections are verified by `Ready`; plugins that need a background loop (RabbitMQ) start it in `Start`.

`Ready`

- Answers "can this plugin serve requests right now", and is **called during startup only**.
- Must be cheap, repeatable and concurrency-safe; no self-retrying, no long blocking, no side effects.
- Unrelated to `/_healthy`: `/_healthy` is liveness, answered by the process itself and never routed through plugins.

`Drain` / `Close`

- The `ctx` they receive is **not** the one passed to `Start`; it is a new, timeout-bearing context created by the lifecycle. Do not use it to decide whether the process is exiting.
- `Drain` = stop accepting new work and wait for in-flight work; `Close` = release resources. The two cannot be merged.

`Name`

- Used in logs and error messages, **never as a registry key** (looking components up by name turns the plugin system into a service locator).

## 5. Lifecycle state machine

```
                    ┌─────────┐
                    │  init   │
                    └────┬────┘
                         │ Start(ctx): concurrent go p.Start(ctx)
                         │ plus p.Ready(ctx) polling, 2s per attempt,
                         │ ready_timeout as the overall budget
              ┌──────────┴───────────┐
              │ all Ready            │ timeout / any Start returns
              ▼                       ▼
        ┌──────────┐            ┌────────────┐
        │ started  │            │  rollback  │ close started plugins
        └────┬─────┘            └────────────┘ in reverse order
             │ ctx.Done, or any Start returns early
             ▼
        ┌──────────┐  state.draining = true, /readyz answers 503 at once
        │ draining │
        └────┬─────┘
             │ Drain in reverse (new ctx, drain_timeout per plugin)
             │ wait for all Starts to return → cancel the Start ctx
             ▼
        ┌──────────┐
        │ closing  │ Close in reverse (new ctx, close_timeout per plugin)
        └────┬─────┘
             ▼
           done
```

### 5.1 Phase semantics

| Phase | Context used | Timeout | Consequence of failure |
|---|---|---|---|
| Start | The caller's ctx, derived by the lifecycle | None (bounded by the Ready timeout) | Roll back started plugins, return the error |
| Ready | 2s derived from `ready_timeout` per call | `ready_timeout` per plugin | Same as above |
| Drain | **New** `context.Background()` | `drain_timeout` per plugin | Record the error, continue with the next; cancelled on timeout |
| Close | **New** `context.Background()` | `close_timeout` per plugin | Record the error, continue with the next |

### 5.2 Ordering rules

- **Start is concurrent**: boot time equals the slowest plugin, not the sum. When partial ordering is needed, plugins hold references to each other and wait directly (for example `http.Start` waits for gRPC's Ready at the beginning); **no global dependency graph is introduced**.
- **Drain is reverse and serial**: HTTP drains first (its in-flight requests still need gRPC), gRPC stops after.
- **Close is reverse and serial**: whatever started last is released first. The list order therefore also determines the close order, and the assembler must put infrastructure first and entry points last.
- **OTEL goes first in the list**: it is the only plugin that must `Shutdown` after every other plugin has closed (to flush remaining spans). The reverse ordering of `[otel, mysql, …, grpc, http]` makes OTEL run last.
- **OTEL is initialised in its constructor**: it registers gRPC interceptors, which must be in place before the gRPC server is constructed. `grpc.NewGRPCPlugin` takes `*tracing.OTELPlugin` as a constructor argument, so wire guarantees OTEL is constructed first.

## 6. Package structure and public API

```
ezboot/
├── plugin.go            Plugin, PluginBase          (public)
├── state.go             State                       (public)
├── app.go               Application, NewApp, options(public)
├── infra/               six infrastructure plugins  (public, new package)
│   ├── mysql.go         MySQL
│   ├── redis.go         Redis
│   ├── kafka.go         Kafka
│   ├── rabbitmq.go      RabbitMQ
│   ├── es.go            Elasticsearch
│   └── etcd.go          Etcd
├── http/                HTTPPlugin                  (public)
├── grpc/                GRPCPlugin                  (public)
├── grpc/client/         GRPCClientPlugin            (public, new package)
├── tracing/             OTELPlugin                  (public, new package)
└── lifecycle.go         state machine (unexported)
```

> The state machine lives in the root package rather than `internal/lifecycle`: it references `Plugin`, which must be public in the root package, and a separate package would create an import cycle.

Public API changes:

> The top-level `grpc/client/` is a newly added public package; it is not the same layer as the existing `internal/gin-boot/grpc_boot`. Internal packages keep implementation details; public packages expose only plugins and narrow accessors.

| Before | Change |
|---|---|
| `ezboot.WithHttpServer` / `WithGrpcServer` | Marked deprecated, internally translated into plugin registration; new code uses `WithPlugins` |
| `ezboot.Healthy` | Removed; readiness now uses `ezboot.State` |
| `ezboot.ProvideRedis` / `ProvideRabbitMQ` / `RabbitMQClient` and similar aliases | Removed, replaced by `ezboot/infra` |
| `boot.Default` / `boot.Server` / `Run` / `DefaultServer` | Removed (about 240 lines of dead code) |
| `Application.Start(ctx)` | Kept, internally changed to run the lifecycle |
| `http.New` / `grpc.New` | Signature unchanged, but no longer calls `boot.Default()`; `net.Listen` moved into `Start` |
| `boot.Infra` / `infra.Set` / `infra.Default()` / `infra.Open` / `infra.Healthy` | Removed |

`http.Server` and `grpc.Server` **do not import the root package**: they implement the 5 methods directly and satisfy `ezboot.Plugin` structurally. Likewise, `http.WithReadyChecker` accepts a locally defined `interface{ Ready() error }`, satisfied by `ezboot.State`. This keeps both transport packages free of an import cycle with the root package.

### 6.1 Plugin inventory

| Plugin | Package | `Start` | `Ready` | `Drain` | `Close` |
|---|---|---|---|---|---|
| `Databases` | `infra/db.go` | Builds pools at construction (single dialect); Start registers the `with` default engine → blocks | Pings all | — | Close pools |
| `Redis` | `infra/redis.go` | Builds clients at construction; Start registers the default → blocks | Ping | — | Close |
| `Kafka` | `infra/kafka.go` | Records brokers at construction; Start blocks | Probe broker | — | Close |
| `RabbitMQ` | `infra/rabbitmq.go` | Builds objects at construction; Start begins reconnecting → blocks | Probe channel | — | Close |
| `Elasticsearch` | `infra/es_search.go` | Builds the client at construction; Start blocks | Ping | — | Close |
| `Etcd` | `infra/etcd.go` | Builds the client at construction; Start blocks | Probe endpoint | — | Close |
| `HTTP` | `http` | `ListenAndServe` (plus a separate pprof listener when configured) | Listener bound | `Shutdown` (including the pprof listener) | — |
| `GRPC` | `grpc` | Register + `Serve` | Listener bound and gateway ready | `GracefulStop` (`Stop` after timeout) | — |
| `GRPCClient` | `grpc/client` | Load client config → blocks | — | — | `CloseClients` |
| `OTEL` | `tracing` | Provider initialised and interceptors registered at construction; Start blocks | — | — | `tp.Shutdown` (flush) |

### 6.2 Default composition

```go
// ezboot/wire.go — only "no customisation needed, always enabled" dependencies and plugins
var ProviderSet = wire.NewSet(
    ProvideConfig,
    ProvideLogger,
    NewState,
    tracing.NewOTEL,
    grpcclient.NewGRPCClient,
)
```

**The http / grpc plugins are deliberately left out of the default set**: they need application-specific inputs such as routes, service registrations and the gateway, and must be built by the application's own providers. Including `grpc.New` / `http.New` here would collide with the application's constructors (wire: multiple bindings).

The infra plugins are **not** in the default composition either: a service that only speaks HTTP should not open Redis just because it uses the framework. `pprof` and `metrics` are not plugins; they are folded into the HTTP and gRPC plugins respectively.

Even though the OTEL plugin is always in the default composition, **whether it does anything is decided by configuration**: with `app.middleware.tracing.startup: false` (the zero value) the constructor does nothing (matching today's `InitOTEL` returning `nil`), and `Close` is a no-op. This is the same pattern as the `GRPCClient` plugin idling when `grpc.client` config is empty.

### 6.3 ProviderSets and native types

Each infra plugin lives with its own file, and that file provides the plugin type, constructors, lifecycle methods and its `ProviderSet` — there is no central `plugin.go`.

The plugin type **is the resource container itself**: `MySQL` holds `mysqls`/`sqlites`/`pgsqls` directly, `Redis` holds clients directly, with no extra `dbConns` / `redisClients` layer.

```go
// infra/redis.go
var RedisProviderSet = wire.NewSet(NewRedis, ProvideDefaultRedis)

func ProvideDefaultRedis(r *Redis) (*redis.Client, error) {
    return r.Redis(defaultName)
}
```

Callers put a single set into wire and business code receives the native type:

```go
wire.Build(
    ezboot.ProviderSet,
    infra.RedisProviderSet,   // *infra.Redis + *redis.Client
    // ...
)

func NewExampleDAO(log *ezboot.Logger, cache *redis.Client) (*ExampleDAO, error)
```

| ProviderSet | Contains | Default provider |
|---|---|---|
| `infra.DatabasesProviderSet` | `NewDatabases` + `ProvideDefaultEngine` (`*xorm.Engine`) | Yes (uniform across the three dialects) |
| `infra.RedisProviderSet` | `NewRedis` + `ProvideDefaultRedis` (`*redis.Client`) | Yes |
| `infra.ElasticsearchProviderSet` | `NewElasticsearch` + `ProvideDefaultElasticsearch` (`*elasticsearch.Client`) | Yes |
| `infra.EtcdProviderSet` | `NewEtcd` + `ProvideDefaultEtcd` (`*clientv3.Client`) | Yes |
| `infra.KafkaProviderSet` | `NewKafka` | No (readers/writers are created per topic) |
| `infra.RabbitMQProviderSet` | `NewRabbitMQ` | No (publishers/consumers are used by name) |

The database plugin requires **exactly one dialect to be configured at a time**: `NewDatabases` validates that exactly one of MySQL / SQLite / Postgres is enabled; zero or several is an error. A single dialect may have multiple named instances (primary, reporting, …), retrieved with `d.Engine("report")`.

All three dialects go through xorm: MySQL uses the `mysql` driver, SQLite `sqlite3`, Postgres `pgx`, and the default instance is always `*xorm.Engine`, so a single `ProvideDefaultEngine` is enough. `with.SetDefault` therefore holds for all three dialects.

### 6.4 What is not a plugin

There is one criterion: **does it have a runtime lifecycle?** Anything with only construction-time actions and no Start/Ready/Drain/Close semantics would become a shell that needs to be captured by `[]Plugin` just to avoid being pruned by wire.

| Thing | Why it is not a plugin | Belongs to |
|---|---|---|
| GinLogger / GinRecovery / sentrygin | No lifecycle, order-sensitive | HTTP plugin constructor |
| apmgrpc / GRPCLogger / Recovery / Validator / Env / Before | Same | gRPC plugin constructor (`GrpcInterceptors`) |
| `grpc_prometheus.Register` + interceptors | Registration at construction only | gRPC plugin constructor |
| `/metrics` route | Route registration at construction only | HTTP plugin constructor |
| `pprof` | A route registration or an extra listener, with no independent startup order | HTTP plugin |
| `_healthy` | Process-level liveness | Built-in route of the HTTP plugin |

pprof keeps today's `shouldPprofRegisterOnBusinessHandler()` logic:

```
pprof_host/pprof_port match the business listener, or both are empty
    → pprof.Register(engine) at construction, routes on the business port
otherwise
    → listen on the pprof port inside Start, Shutdown together during Drain
```

**Note that this is a behaviour change**: `ensurePprofHandlers()` previously lived only in the dead `Run()` and never executed. After folding it into the HTTP plugin, pprof takes effect for the first time, exposing `/debug/pprof` on the business port by default.

**Rule: anything that must happen before serving belongs in the constructor; runtime behaviour belongs in Start.** wire guarantees construction order (whoever is depended on is built first); the lifecycle is not responsible for construction order. This is also why `promhttp.Handler()` for `/metrics` and `grpc_prometheus.Register` need no dependency between them — the handler reads the registry at scrape time.

The metrics switch reuses the already existing but previously unread `app.middleware.monitor`:

```yaml
app:
  middleware:
    monitor:
      _startup: true     # absent = off, matching today's behaviour
      listen: ""         # empty = business port; non-empty = dedicated port
      metric: "/metrics"
```

### 6.5 Shared values

Two values belong to no plugin but must be injected into several, so they are standalone providers:

- `*ezboot.State`: created by `NewState()` before any plugin; written by the lifecycle, read by the HTTP plugin.
- `*gin.Engine`: the HTTP routing table. pprof registers routes on it and the HTTP plugin uses it as its handler. Created by `http.NewEngine(cfg)`.

The gRPC driver is created by the `grpc` plugin in its constructor and exposed via `Driver()` for metrics and service registration; `net.Listen` moved into `Start`.

## 7. Configuration

### 7.1 Lifecycle

```yaml
app:
  lifecycle:
    ready_timeout: 10s   # startup, per plugin
    drain_timeout: 10s   # drain, per plugin
    close_timeout: 5s    # release, per plugin
```

Mapped to `conf_type.Lifecycle` (`custom.YAMLDuration`, supporting `10s` / `500ms`) and passed in by `ezboot.WithLifecycleConfig(cfg)`. Zero values fall back to the framework defaults. Operators must ensure:

```
terminationGracePeriodSeconds > Σ(drain_timeout) + Σ(close_timeout) + margin
```

The chart needs `terminationGracePeriodSeconds` and `readinessProbe.timeoutSeconds: 3`.

### 7.2 Official plugins

They keep using `app.middleware.*` in `<env>.yaml`, strictly decoded by `config.Config` (`SetStrict(true)` at `config/config.go:78`).

### 7.3 User plugins

User plugin configuration goes into `<env>_custom.yaml` and is read with the caller's own struct:

```go
type KafkaConsumerConfig struct {
    Topic string `yaml:"topic"`
    Group string `yaml:"group"`
}

func NewKafkaConsumer(cfg *KafkaConsumerConfig) *KafkaConsumer
```

Two things matter for `LoadUserConfig`:

1. It no longer calls `boot.Default()` and depends only on the config-loading step (see §8).
2. It checks `mapstructure.Metadata.Unused` and warns when non-empty — otherwise a typo in user plugin config is silent.

**Known asymmetry**: `_custom.yaml` only enters global viper and does not affect `config.Config` (it is not an override file). Framework fields written into custom files neither take effect nor raise errors, and this needs to be documented.

## 8. Assembly

### 8.1 Splitting up boot.Default()

```
LoadFrameworkConfig()   env + viper + logger + strict decode + rewrite + custom
InitTelemetry()         OTEL (idempotent) + gRPC client pool config
Migrate()               auto-migration, called by Application before plugins start
```

`Default()` / `Server` / `Run` are gone. `ProvideConfig` performs `LoadFrameworkConfig` + `InitTelemetry`: the gRPC driver reads the interceptor list when constructed, so tracing must run before `grpc.New`.

### 8.2 wire

```go
// framework
var ProviderSet = wire.NewSet(
    ProvideConfig,
    ProvideLogger,
    NewState,
    tracing.NewOTEL,
    grpcclient.NewGRPCClient,
)

// caller (http / grpc are built by the application)
func NewPlugins(
    otel *tracing.OTEL,
    db *infra.Databases,
    gc *grpcclient.GRPCClient,
    gs *grpc.Server,
    hs *http.Server,
) []ezboot.Plugin {
    // order = basis for reverse Close: otel first → flushed last
    return []ezboot.Plugin{otel, db, gc, gs, hs}
}

func NewApp(cfg *config.Config, state *ezboot.State, ps []ezboot.Plugin, commands cli.Commands) (*ezboot.Application, error) {
    return ezboot.NewApp(
        ezboot.WithAppName(cfg.Name),
        ezboot.WithState(state),
        ezboot.WithPlugins(ps...),
        ezboot.WithCmdline(commands...),
    )
}
```

wire does not support multiple bindings of the same type, so `[]Plugin` must be collapsed by one explicit function. That is deliberate: adding a plugin means changing the signature and regenerating.

### 8.3 Entrypoint

```go
ctx, cancel := signal.WithExit(context.Background())
defer cancel()

if err := app.Start(ctx); err != nil {
    panic(err)
}
```

The signal is owned by the caller (S1). `app.Start(ctx)` returns only after drain + close complete.

### 8.4 Readiness endpoint

```go
http.NewHTTPPlugin(cfg, engine, state, http.WithReady(state.Ready))

// runtime: O(1), touches no plugin
func (s *State) Ready() error {
    if !s.started.Load()  { return errors.New("ezboot: not started") }
    if s.draining.Load()  { return errors.New("ezboot: draining") }
    return nil
}
```

## 9. Migration plan

| Phase | Content | Mergeable independently |
|---|---|---|
| 1 | `plugin.go` + `state.go` + unit tests | Yes |
| 2 | `lifecycle.go` state machine + fake-plugin tests | Yes |
| 3 | Split `boot.Default()`; remove globals from `LoadUserConfig` | No, together with 4 |
| 4 | Turn the six `infra` components into plugins; delete `Set` / `boot.Infra` | No, together with 3 |
| 5 | Turn `http` / `grpc` into plugins; move `net.Listen` into Start | Yes |
| 6 | Connect `Application` to the lifecycle; delete `boot.Server.Run` | Yes |
| 7 | Migrate template + chart | With 6 |
| 8 | `OTEL` / `GRPCClient` plugins; metrics into the grpc / http plugins; pprof into the http plugin | Yes |

Phases 3 and 4 must land together: once `boot.Default()` no longer calls `infra.Open`, `ProvideRedis` / `ProvideRabbitMQ` / `Healthy`, which depended on it, lose their data source. During implementation those three first degraded to returning `ErrResourceUnavailable`; Phase 8 removed them.

**Progress**: Phases 1–8 are complete.

- Phase 1–2: `plugin.go`, `state.go`, `lifecycle.go`, `lifecycle_test.go`.
- Phase 3: `boot.LoadFrameworkConfig()`; `LoadUserConfig` no longer triggers bootstrap.
- Phase 4: `internal/gin-boot/infra` moved to the public `infra` package and `Set` split into six plugins; `boot.Infra`, `infra.Open` and `infra.Healthy` deleted. `with` now gets its default handles registered by the MySQL / Redis plugins.
- Phase 5: `http.Server` / `grpc.Server` implement `Plugin`; `net.Listen` moved into `Start`; `http.New` no longer depends on `boot.Default()` and ships its own `NewEngine`; pprof folded into the HTTP plugin.
- Phase 6: `Application` uses the lifecycle; `boot.Migrate()` runs before plugins start; `boot.InitTelemetry()` is called by `ProvideConfig` so gRPC interceptors exist before the driver is constructed.
- Phase 7: template migration — `NewPlugins` collapses `[]ezboot.Plugin`, `ezboot.State` drives `/readyz`, the MySQL plugin is wired in, and the chart gains `terminationGracePeriodSeconds` and a readiness probe. The template also ships a complete custom-plugin example under `internal/plugin/example`: `_custom.yaml` config, all five phases, and a unit test that runs without starting the application.
- Phase 8: `tracing.OTEL` and `grpc/client.GRPCClient` plugins; metrics folded into http / grpc.
- Cleanup: `boot.Default` / `Server` / `Run` and `internal/gin-boot/internal/cmd` deleted; `boot.go` shrank from 469 lines to 225.

Kafka's broker handle type was renamed from `Kafka` to `KafkaBrokers`, leaving `Kafka` for the plugin.

**Removed**: the entire old orchestration (`boot.Default()` / `boot.Server` / `Run` / `DefaultServer`) plus `internal/gin-boot/internal/cmd`, which existed only to serve it.

**Acceptance**

- Lifecycle unit tests cover: concurrent startup, Ready timeout, Start-failure rollback, Drain/Close ordering, and Drain receiving a new ctx.
- `go build ./...`, `go vet ./...`, `go test ./...` (except the pre-existing `mockutil` failure).
- The template builds with a local replace.
- Manual test: send SIGTERM while requests are in flight; `/readyz` returns 503 first, in-flight requests complete normally.

## 10. Decision log (rejected alternatives)

| Alternative | Why it was rejected |
|---|---|
| Runtime plugin loading (`plugin` package / separate process) | Unusable with cross-compilation and container images; wire stops working entirely |
| Each plugin declaring `DependsOn() []string` plus a topological sort | Moves ordering knowledge out of the type system and into strings; ordering needs are already covered by "concurrent start + readiness" and by plugins referencing each other directly |
| One plugin per configured instance (`MySQL("default")`, `MySQL("legacy")`) | wire needs same-type deps with different names; `Startup` already decides "which instances are open" at the config layer |
| `/readyz` re-probing every dependency at runtime | A shared dependency failure would make every Pod NotReady at once, amplifying a local degradation into a global outage; it also needs a concurrent probe budget inside the request path, costing more than it returns. Runtime readiness reflects only `started && !draining` |
| Splitting into two methods, `Ready` / `Health` | Identical implementations, forcing plugin authors to choose; one `Ready` suffices, and the difference lies in how callers react |
| Keeping `infra.Set` as a single plugin | Reintroduces the "bag of resources"; single-resource plugins get startup rollback for free |
| Keeping `boot.Default()` as the composition root | Official plugins would get a privileged path users do not have, violating "one assembly path" |
| `GRPCPrometheus` as its own plugin | No lifecycle, only construction-time registration; a production necessity should be the default behaviour, not something callers must remember to add |
| `PPROF` as its own plugin | No independent startup order — just a route registration or an attached listener; folded into the HTTP plugin, its lifecycle is covered naturally |
| OTEL folded into the gRPC plugin | HTTP also uses otelgin, so ownership would be awkward; and the provider's flush must happen after every plugin, so it must be its own phase |
| `Application` listening for signals itself | An embeddable framework should not monopolise process-level signals; `signal.WithExit` is already public API |

## 11. Risks and known limitations

| Item | Description | Mitigation |
|---|---|---|
| Timing of `with.SetDefault` | The MySQL plugin sets the global engine in `Start`, so DAOs are usable only after ready | Migration does not use that global (it opens its own connection) |
| The gateway handler is produced inside gRPC.Start | HTTP requests arriving before gRPC is ready get a 503 | `/readyz` returns 503 before ready; optionally `http.Start` waits locally for gRPC |
| Close order depends on list order | A mistaken assembly order closes infrastructure first | Ordering documented; the template gives a reference order |
| Global viper / config.Conf | Multiple instances in one process remain impossible | Future work |
| `_custom.yaml` bypasses strict validation | Misspelled user plugin fields are silent | `Metadata.Unused` warnings |
| pprof takes effect for the first time | `ensurePprofHandlers` used to sit in dead code; after folding into the HTTP plugin, `/debug/pprof` is exposed on the business port by default | Check `pprof_host` / `pprof_port` before deploying; add an explicit switch if it must be off |
| OTEL must be first in the list | Reverse close makes it run last, so it can flush remaining spans; a wrong position loses tail traces | Documented in §5.2; the template gives a reference order |

## 12. Settled decisions

- Interface naming: `PluginBase`.
- Default composition: `OTEL` + `GRPC` + `HTTP` + `GRPCClient`. OTEL is always in the composition, and `tracing.startup` decides whether it actually does anything.
- `PPROF`: not a plugin; folded into the HTTP plugin, keeping the `shouldPprofRegisterOnBusinessHandler()` logic.
- metrics: not a standalone plugin; folded into the grpc / http plugins, controlled by `app.middleware.monitor._startup`, off by default.
- User plugin config: `<env>_custom.yaml` plus the caller's own struct.
