# ezboot 插件化与生命周期设计

> 状态：Draft（2026-10-28）
> 范围：`internal/gin-boot/boot.go`、`app.go`、`http/`、`grpc/`、`internal/gin-boot/infra/`、wire 与 template 装配方式。
> 本文只描述设计与迁移路径，不含实现。

## 1. 背景

当前框架有三个互相纠缠的问题：

1. **组合根封闭。** 启动流程写死在 `boot.Default()` 和 `boot.Server.Run()` 里，新增一个组件（websocket hub、consumer、admin server）必须改框架内部。
2. **构造期做 I/O。** `http.New` 调 `boot.Default()`，触发 config → migration → `infra.Open` → OTEL → gRPC client 全部执行。组件实际上是全局单例的视图，无法在一个进程里构造两个应用。
3. **两套启动路径，活的那套不完整。** `Server.Run()`（`internal/gin-boot/boot.go:315`）包含 pprof、grpc_prometheus、后台 handler、`OnShutdown`（关 gRPC client、关 infra）——但它在仓库里**没有任何调用方**。真实路径是 `Application.serve`（`app.go:34`），只并发启动 HTTP/gRPC，没有 Close，没有 drain 超时，没有 infra 释放。

因此本设计的目标不是"让组件长得一样"，而是**把组合根从框架内部搬到调用方**，并让启动/停止成为一个有明确阶段、明确超时、可回滚的状态机。

## 2. 目标 / 非目标

**目标**

- 组件（官方与用户编写）实现同一接口，由调用方在 wire 中显式选择。
- 启动与停止有明确阶段、超时和错误传播；失败可回滚。
- 官方插件与用户插件走完全相同的装配路径（官方无特权）。
- 删除死代码，消除 `boot.Default()` 作为组合根的角色。

**非目标**

- 运行时插件加载（`plugin` 包 / 独立进程 / RPC 扩展）。Go 交叉编译与容器镜像场景下代价过高，本次明确不做。
- 同进程多实例。本设计移除了 `infra` 全局，但 `viper`、`config.Conf`、`gin.SetMode`、logging 默认实例仍是包级状态，多实例不在本次范围。
- 运行期依赖探活（见 §10 决策记录）。

## 3. 核心概念

| 概念 | 说明 |
|---|---|
| Plugin | 一个参与应用生命周期的组件，5 个方法 |
| PluginBase | 提供 4 个阶段的空实现，嵌入后只需实现 `Name` |
| State | `started` / `draining` 两个原子 bool，`/readyz` 唯一的数据源 |
| Lifecycle | 阶段机的实现，内部包，不导出 |

## 4. 接口

```go
// Plugin 是应用生命周期中一个可插拔的组件。
// 阶段调用顺序：Start → 轮询 Ready → Drain → Close。
type Plugin interface {
    Name() string
    Start(ctx context.Context) error
    Ready(ctx context.Context) error
    Drain(ctx context.Context) error
    Close(ctx context.Context) error
}

// PluginBase 提供四个阶段的默认实现。嵌入后只需要实现 Name，
// 以及你真正关心的那几个阶段。
type PluginBase struct{}

// Start 阻塞到 ctx 结束。Start 返回就代表插件已经停止，
// lifecycle 依靠这个信号判断插件是否还活着。
func (PluginBase) Start(ctx context.Context) error { <-ctx.Done(); return nil }

// Ready 默认认为插件随时可用。有连接语义的插件必须覆盖它。
func (PluginBase) Ready(context.Context) error { return nil }

func (PluginBase) Drain(context.Context) error { return nil }
func (PluginBase) Close(context.Context) error { return nil }
```

### 4.1 契约

`Start`

- 阻塞直到插件停止。**覆盖 `Start` 必须自己写 `<-ctx.Done()`**，否则会被判定为"刚启动就退出"。
- 返回 `nil` 表示正常停止，返回 error 表示故障。
- I/O 一律放在 `Start`，构造函数只做配置校验。

`Ready`

- 回答"这个插件现在能不能提供服务"，**只在启动期被调用**。
- 必须便宜、可重复调用、并发安全；不得自己重试、不得长时间阻塞、不得有副作用。
- 与 `/_healthy` 无关：`/_healthy` 是 liveness，只由进程自己回答，不经过插件。

`Drain` / `Close`

- 收到的 ctx **不是** `Start` 时的那个，而是 lifecycle 新建的、带超时的 ctx。不要用它判断进程是否要退出。
- `Drain` = 停止接受新工作、等待在途工作结束；`Close` = 释放资源。两者不可合并。

`Name`

- 用于日志与错误信息，**不作为 registry key**（按名字查找组件会把插件系统变成 service locator）。

## 5. 生命周期状态机

```
                    ┌─────────┐
                    │  init   │
                    └────┬────┘
                         │ Start(ctx)：并发 go p.Start(ctx)
                         │ 并轮询 p.Ready(ctx)，单次派生 2s，总预算 ready_timeout
              ┌──────────┴───────────┐
              │ 全部 Ready            │ 超时 / 任一 Start 返回
              ▼                       ▼
        ┌──────────┐            ┌────────────┐
        │ started  │            │  rollback  │ 逆序 Close 已启动的插件
        └────┬─────┘            └────────────┘
             │ ctx.Done 或任一 Start 提前返回
             ▼
        ┌──────────┐  state.draining = true，/readyz 立即 503
        │ draining │
        └────┬─────┘
             │ 逆序 Drain（新 ctx，drain_timeout/插件）
             │ 等所有 Start 返回 → cancel Start 的 ctx
             ▼
        ┌──────────┐
        │ closing  │ 逆序 Close（新 ctx，close_timeout/插件）
        └────┬─────┘
             ▼
           done
```

### 5.1 阶段语义

| 阶段 | 使用的 ctx | 超时 | 失败后果 |
|---|---|---|---|
| Start | 调用方传入的 ctx（由生命周期派生） | 无（由 Ready 超时兜底） | 回滚已启动插件，返回错误 |
| Ready | 每次调用从 `ready_timeout` 派生 2s | 每插件 `ready_timeout` | 同上 |
| Drain | **新建** `context.Background()` | 每插件 `drain_timeout` | 记录错误，继续下一个；超时后 cancel |
| Close | **新建** `context.Background()` | 每插件 `close_timeout` | 记录错误，继续下一个 |

### 5.2 顺序规则

- **Start 并发**：启动耗时等于最慢的插件，而不是所有插件之和。需要局部顺序时，插件之间直接持有引用自行等待（例如 `http.Start` 开头等 gRPC 的 Ready），**不引入全局依赖图**。
- **Drain 逆序串行**：HTTP 先排空（它的在途请求还要发给 gRPC），gRPC 后停。
- **Close 逆序串行**：最后启动的最先释放。因此列表顺序同时决定了关闭顺序，装配方需要把"基础设施"放前面、"入口"放后面。
- **OTEL 放列表最前**：它是唯一需要在其他所有插件关闭之后才 `Shutdown` 的插件（flush 剩余 span）。列表 `[otel, mysql, …, grpc, http]` 的逆序关闭正好让 OTEL 最后执行。
- **OTEL 的初始化在构造函数里完成**：它要注册 gRPC 拦截器，而拦截器必须在 gRPC server 构造前就位。`grpc.NewGRPCPlugin` 把 `*tracing.OTELPlugin` 作为构造参数，wire 因此保证 OTEL 先构造。

## 6. 包结构与公共 API

```
ezboot/
├── plugin.go            Plugin, PluginBase          （公共）
├── state.go             State                       （公共）
├── app.go               Application, NewApp, options（公共）
├── infra/               六个基础设施插件             （公共，新包）
│   ├── mysql.go         MySQL
│   ├── redis.go         Redis
│   ├── kafka.go         Kafka
│   ├── rabbitmq.go      RabbitMQ
│   ├── es.go            Elasticsearch
│   └── etcd.go          Etcd
├── http/                HTTPPlugin                  （公共）
├── grpc/                GRPCPlugin                  （公共）
├── grpc/client/         GRPCClientPlugin            （公共，新包）
├── tracing/             OTELPlugin                  （公共，新包）
└── lifecycle.go         状态机实现（未导出）
```

> 状态机放在根包内，而不是 `internal/lifecycle`：它要引用 `Plugin`，而 `Plugin` 必须公开在根包，独立包会形成循环依赖。

公开 API 变化：

> 顶层的 `grpc/client/` 是新增的公共包，与现有的 `internal/gin-boot/grpc_boot` 不是同一层。内部包保留实现细节，公共包只放插件与窄访问器。

| 现有 | 变化 |
|---|---|
| `ezboot.WithHttpServer` / `WithGrpcServer` | 标记 deprecated，内部转为插件注册；新代码用 `WithPlugins` |
| `ezboot.Healthy` | 删除；就绪状态改用 `ezboot.State` |
| `ezboot.ProvideRedis` / `ProvideRabbitMQ` / `RabbitMQClient` 等别名 | 删除，改用 `ezboot/infra` |
| `boot.Default` / `boot.Server` / `Run` / `DefaultServer` | 删除（约 240 行死代码） |
| `Application.Start(ctx)` | 保留，内部改为运行 lifecycle |
| `http.New` / `grpc.New` | 签名不变，但不再调用 `boot.Default()`；`net.Listen` 移到 `Start` |
| `boot.Infra` / `infra.Set` / `infra.Default()` / `infra.Open` / `infra.Healthy` | 删除 |

`http.Server` 与 `grpc.Server` **不 import 根包**：它们直接实现 5 个方法，靠 Go 的结构化接口满足 `ezboot.Plugin`。同理，`http.WithReadyChecker` 接受一个本地定义的 `interface{ Ready() error }`，由 `ezboot.State` 满足。这样两个传输包都不会与根包形成循环依赖。

### 6.1 插件清单

| 插件 | 包 | `Start` | `Ready` | `Drain` | `Close` |
|---|---|---|---|---|---|
| `MySQL` | `infra` | 开 pool → 阻塞 | Ping default | — | 关 pool |
| `Redis` | `infra` | 建 client → 阻塞 | Ping | — | 关闭 |
| `Kafka` | `infra` | 建 reader/writer 容器 → 阻塞 | 探 broker | — | 关 reader/writer |
| `RabbitMQ` | `infra` | 建连接 → 阻塞 | 探 channel | — | 关闭 |
| `Elasticsearch` | `infra` | 建 client → 阻塞 | Ping | — | 关闭 |
| `Etcd` | `infra` | 建 client → 阻塞 | 探 endpoint | — | 关闭 |
| `HTTP` | `http` | `ListenAndServe`（pprof 独立端口时同时 Serve） | listener 已绑定 | `Shutdown`（含 pprof 监听） | — |
| `GRPC` | `grpc` | 注册 + `Serve` | listener 已绑定且 gateway 就绪 | `GracefulStop`（超时后 `Stop`） | — |
| `GRPCClient` | `grpc/client` | 载入 client 配置 → 阻塞 | — | — | `CloseClients` |
| `OTEL` | `tracing` | 构造期初始化 provider、注册拦截器；Start 阻塞 | — | — | `tp.Shutdown`（flush） |

### 6.2 默认组合

```go
// ezboot/wire.go
var DefaultProviderSet = wire.NewSet(
    tracing.NewOTEL,
    grpc.New,
    http.New,
    grpcclient.NewGRPCClient,
)
```

infra 插件**不在**默认组合中：只用 HTTP 的服务不应该因为引入了框架就打开 Redis。`pprof` 与 `metrics` 不是插件，分别并进 HTTP / gRPC 插件。

OTEL 插件虽然恒在默认组合里，但**实际是否启用由配置决定**：`app.middleware.tracing.startup: false`（零值）时构造函数什么都不做（与今天 `InitOTEL` 返回 `nil` 一致），`Close` 也是空操作。这和 `grpc.client` 配置为空时 `GRPCClient` 插件空转是同一种模式。

### 6.3 什么不是插件

判断标准只有一条：**有没有运行期生命周期**。只有构造期动作、没有 Start/Ready/Drain/Close 语义的东西，做成插件只会得到一个需要被 `[]Plugin` 收口才不会被 wire 裁掉的空壳。

| 东西 | 为什么不是插件 | 归属 |
|---|---|---|
| GinLogger / GinRecovery / sentrygin | 无生命周期，顺序敏感 | HTTP 插件构造函数 |
| apmgrpc / GRPCLogger / Recovery / Validator / Env / Before | 同上 | gRPC 插件构造函数（`GrpcInterceptors`） |
| `grpc_prometheus.Register` + 拦截器 | 只有构造期注册 | gRPC 插件构造函数 |
| `/metrics` 路由 | 只有构造期注册路由 | HTTP 插件构造函数 |
| `pprof` | 路由注册或一个额外 listener，没有独立启动顺序 | HTTP 插件 |
| `_healthy` | 进程级 liveness | HTTP 插件内置路由 |

pprof 的启用逻辑沿用今天的 `shouldPprofRegisterOnBusinessHandler()`：

```
pprof_host/pprof_port 与业务一致，或两者都为空
    → 构造期 pprof.Register(engine)，路由挂在业务端口
否则
    → Start 里单独监听 pprof 端口，Drain 时一起 Shutdown
```

**注意这是行为变化**：`ensurePprofHandlers()` 之前只在死代码 `Run()` 里，从未真正执行。并进 HTTP 插件后 pprof 会第一次真正生效，默认挂在业务端口的 `/debug/pprof` 下。

**规则：必须在 serve 之前完成的动作放构造函数，运行期行为放 Start。** wire 保证构造顺序（依赖谁就先构造谁），lifecycle 不负责构造顺序。这也是为什么 `/metrics` 的 `promhttp.Handler()` 可以和 `grpc_prometheus.Register` 互不依赖——handler 在每次抓取时才读注册表。

metrics 开关复用已存在但当前无人读取的 `app.middleware.monitor`：

```yaml
app:
  middleware:
    monitor:
      _startup: true     # 不填 = 关闭，与今天行为一致
      listen: ""         # 空 = 挂在业务端口；非空 = 独立端口
      metric: "/metrics"
```

### 6.4 共享值

有两个值不属于任何插件，但需要被多个插件注入，因此作为独立 provider：

- `*ezboot.State`：`NewState()` 先于所有插件创建，lifecycle 写、HTTP 插件读。
- `*gin.Engine`：HTTP 路由表。pprof 需要往它上面注册路由，HTTP 插件需要拿它做 handler。由 `http.NewEngine(cfg)` 创建。

gRPC 的 driver 由 `grpc` 插件在构造函数里创建、以 `Driver()` 暴露，供 metrics 注册与 service 注册使用；`net.Listen` 移到 `Start`。

## 7. 配置

### 7.1 生命周期

```yaml
app:
  lifecycle:
    ready_timeout: 10s   # 启动期，每个插件
    drain_timeout: 10s   # 排空，每个插件
    close_timeout: 5s    # 释放，每个插件
```

对应 `conf_type.Lifecycle`（`custom.YAMLDuration`，支持 `10s` / `500ms`），由 `ezboot.WithLifecycleConfig(cfg)` 传入。零值取框架默认。运维需要保证：

```
terminationGracePeriodSeconds > Σ(drain_timeout) + Σ(close_timeout) + 余量
```

chart 需补 `terminationGracePeriodSeconds` 与 `readinessProbe.timeoutSeconds: 3`。

### 7.2 官方插件

沿用 `<env>.yaml` 的 `app.middleware.*`，由 `config.Config` 严格解码（`config/config.go:78` 的 `SetStrict(true)`）。

### 7.3 用户插件

用户插件配置写 `<env>_custom.yaml`，用自己的结构体读取：

```go
type KafkaConsumerConfig struct {
    Topic string `yaml:"topic"`
    Group string `yaml:"group"`
}

func NewKafkaConsumer(cfg *KafkaConsumerConfig) *KafkaConsumer
```

`LoadUserConfig` 需要注意两点：

1. 不再调用 `boot.Default()`，只依赖配置加载步骤（见 §8）。
2. 检查 `mapstructure.Metadata.Unused`，非空时告警——否则用户插件配置拼写错误是静默的。

**已知不对称**：`_custom.yaml` 只进入全局 viper，不影响 `config.Config`（它不是 override 文件）。写进 custom 的框架字段不会生效，也不会报错，需要写进文档。

## 8. 装配

### 8.1 拆分 boot.Default()

```
LoadFrameworkConfig()   env + viper + logger + strict decode + rewrite + custom
InitTelemetry()         OTEL（幂等）+ gRPC 客户端池配置
Migrate()               自动迁移，由 Application 在插件启动前调用
```

`Default()` / `Server` / `Run` 已删除。`ProvideConfig` 负责 `LoadFrameworkConfig` + `InitTelemetry`：gRPC driver 在构造时读取拦截器列表，所以 tracing 必须早于 `grpc.New`。

### 8.2 wire

```go
// 框架
var DefaultProviderSet = wire.NewSet(
    tracing.NewOTEL,
    grpc.New,
    http.New,
    grpcclient.NewGRPCClient,
)

// 调用方
func NewPlugins(
    otel *tracing.OTEL,
    db *infra.MySQL,
    gc *grpcclient.GRPCClient,
    gs *grpc.Server,
    hs *http.Server,
) []ezboot.Plugin {
    // 顺序 = Close 的逆序依据：otel 最前 → 最后 flush
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

wire 不支持同类型多绑定，`[]Plugin` 必须由一个显式函数收口。这是刻意的：加插件等于改签名 + 重新生成。

### 8.3 入口

```go
ctx, cancel := signal.WithExit(context.Background())
defer cancel()

if err := app.Start(ctx); err != nil {
    panic(err)
}
```

信号由调用方持有（S1）。`app.Start(ctx)` 直到 drain + close 完成才返回。

### 8.4 就绪端点

```go
http.NewHTTPPlugin(cfg, engine, state, http.WithReady(state.Ready))

// 运行期：O(1)，不碰任何插件
func (s *State) Ready() error {
    if !s.started.Load()  { return errors.New("ezboot: not started") }
    if s.draining.Load()  { return errors.New("ezboot: draining") }
    return nil
}
```

## 9. 迁移计划

| Phase | 内容 | 可独立合入 |
|---|---|---|
| 1 | `plugin.go` + `state.go` + 单测 | 是 |
| 2 | `lifecycle.go` 状态机 + 假插件测试 | 是 |
| 3 | 拆 `boot.Default()`；`LoadUserConfig` 去全局 | 否，与 4 一起 |
| 4 | `infra` 六件插件化，删 `Set` / `boot.Infra` | 否，与 3 一起 |
| 5 | `http` / `grpc` 插件化，`net.Listen` 移到 Start | 是 |
| 6 | `Application` 接 lifecycle，删 `boot.Server.Run` | 是 |
| 7 | template + chart 迁移 | 随 6 |
| 8 | `OTEL` / `GRPCClient` 插件；metrics 集成进 grpc / http 插件；pprof 集成进 http 插件 | 是 |

Phase 3 与 4 必须一起落地：`boot.Default()` 一旦不再 `infra.Open`，依赖它的 `ProvideRedis` / `ProvideRabbitMQ` / `Healthy` 就失去数据源。实现时这三者先退化为返回 `ErrResourceUnavailable`，Phase 8 已将它们删除。

**进度**：Phase 1–8 已完成。

- Phase 1–2：`plugin.go`、`state.go`、`lifecycle.go`、`lifecycle_test.go`。
- Phase 3：`boot.LoadFrameworkConfig()`，`LoadUserConfig` 不再触发 bootstrap。
- Phase 4：`internal/gin-boot/infra` 迁到公共包 `infra`，`Set` 拆成六个插件；`boot.Infra`、`infra.Open`、`infra.Healthy` 删除。`with` 改为由 MySQL / Redis 插件注册默认句柄。
- Phase 5：`http.Server` / `grpc.Server` 实现 `Plugin`；`net.Listen` 移到 `Start`；`http.New` 不再依赖 `boot.Default()`，自带 `NewEngine`；pprof 并入 HTTP 插件。
- Phase 6：`Application` 改用 lifecycle；`boot.Migrate()` 在启动插件前执行；`boot.InitTelemetry()` 由 `ProvideConfig` 调用，保证 gRPC 拦截器早于 driver 构造。
- Phase 7：template 迁移 —— `NewPlugins` 收口 `[]ezboot.Plugin`、`ezboot.State` 接入 `/readyz`、MySQL 插件、chart 的 `terminationGracePeriodSeconds` 与 readiness 探针。
- Phase 8：`tracing.OTEL` 与 `grpc/client.GRPCClient` 插件；`ezboot.DefaultProviderSet`；metrics 集成进 http / grpc。
- 清理：`boot.Default` / `Server` / `Run` 与 `internal/gin-boot/internal/cmd` 删除，`boot.go` 从 469 行降到 225 行。

Kafka 的 broker 句柄类型从 `Kafka` 改名为 `KafkaBrokers`，`Kafka` 这个名字留给插件。

**已删除**：`boot.Default()` / `boot.Server` / `Run` / `DefaultServer` 整块旧编排，以及只服务它们的 `internal/gin-boot/internal/cmd`。

**验收**

- lifecycle 单测覆盖：并发启动、Ready 超时、Start 失败回滚、Drain/Close 顺序、Drain 使用新 ctx。
- `go build ./...`、`go vet ./...`、`go test ./...`（既有 `mockutil` 失败除外）。
- template 本地 replace 可构建。
- 手测：在途请求期间发 SIGTERM，`/readyz` 先返回 503，在途请求正常完成。

## 10. 决策记录（被否掉的方案）

| 方案 | 否掉理由 |
|---|---|
| 运行时插件加载（`plugin` 包 / 独立进程） | 交叉编译与容器镜像不可用；wire 完全失效 |
| 每个插件带 `DependsOn() []string` + 拓扑排序 | 把顺序知识从类型系统搬进字符串；顺序需求已由"并发启动 + readiness"和"插件间直接引用"覆盖 |
| 每个配置实例一个插件（`MySQL("default")`、`MySQL("legacy")`） | wire 需要同类型不同名的依赖；`Startup` 已在配置层解决"开哪些实例" |
| `/readyz` 运行期重新探测所有依赖 | 共享依赖故障时全部 Pod 同时 NotReady，把局部降级放大为全局不可用；且需要请求路径内的并发探测预算，成本高于收益。运行期只反映 `started && !draining` |
| 拆成 `Ready` / `Health` 两个方法 | 实现相同，插件作者需要分辨；一个 `Ready` 已足够，差异在调用方如何反应 |
| 保留 `infra.Set` 作为一个插件 | 会重新引入"资源袋"；单资源插件天然获得启动回滚 |
| 保留 `boot.Default()` 作为组合根 | 官方插件会有用户插件没有的特权通道，违背"同一装配路径" |
| `GRPCPrometheus` 做成独立插件 | 没有生命周期，只有构造期注册；生产必需的能力应当是默认行为，而不是要求调用方记得加 |
| `PPROF` 做成独立插件 | 没有独立启动顺序，只是一个路由注册或一个附带 listener；并进 HTTP 插件后生命周期自然被覆盖 |
| OTEL 并进 gRPC 插件 | HTTP 也用 otelgin，归属不自然；且 provider 的 flush 需要晚于所有插件，必须是独立阶段 |
| `Application` 自己监听信号 | 可嵌入的框架不应独占进程级信号；`signal.WithExit` 已是公共 API |

## 11. 风险与已知限制

| 项 | 说明 | 缓解 |
|---|---|---|
| `with.SetDefault` 时机 | MySQL 插件在 `Start` 里设置全局 engine，DAO 只在 ready 后可用 | migration 不使用该全局（自己开连接） |
| gateway handler 在 gRPC.Start 生成 | HTTP 在 gRPC 就绪前收到请求会得到 503 | `/readyz` 在 ready 前 503；必要时 `http.Start` 局部等待 gRPC |
| Close 顺序依赖列表顺序 | 装配方排错顺序会导致先关基础设施 | 文档约定顺序；模板给出参考顺序 |
| 全局 viper / config.Conf | 同进程多实例仍不可行 | 后续工作 |
| `_custom.yaml` 不参与 strict 校验 | 用户插件字段拼错静默 | `Metadata.Unused` 告警 |
| pprof 第一次真正生效 | 之前 `ensurePprofHandlers` 在死代码里，并入 HTTP 插件后默认在业务端口暴露 `/debug/pprof` | 上线前确认 `pprof_host` / `pprof_port`；需要关闭时补显式开关 |
| OTEL 必须排在列表最前 | 逆序关闭时它会最后执行，才能 flush 剩余 span；排错位置会丢尾部 trace | §5.2 已写明；模板给出参考顺序 |

## 12. 已决事项

- 接口命名：`PluginBase`。
- 默认组合：`OTEL` + `GRPC` + `HTTP` + `GRPCClient`。OTEL 恒在组合内，由 `tracing.startup` 控制是否实际启用。
- `PPROF`：不做插件，并进 HTTP 插件；启用逻辑沿用 `shouldPprofRegisterOnBusinessHandler()`。
- metrics：不单独成插件，集成进 grpc / http 插件；由 `app.middleware.monitor._startup` 控制，默认关闭。
- 用户插件配置：`<env>_custom.yaml` + 自己的 struct。
