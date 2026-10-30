# ezboot

**一个生命周期可推理的 Go 分布式服务框架。**

[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Release](https://img.shields.io/badge/release-v0.5.1-blue)](https://github.com/ez4bk/ezboot/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-green)](LICENSE)

简体中文 · [English](README.md)

ezboot 是一个用来构建分布式服务的 Go 框架。组成一个进程的一切——HTTP 服务、gRPC 服务、MySQL、Redis、Kafka、OpenTelemetry——都是实现同一套五个方法的**插件**。一个小状态机并发启动它们，等它们真正就绪，并在退出时按逆序排空、逆序释放。

> [!WARNING]
> ezboot 仍处于 1.0 之前（`v0.5.x`），公开 API 在次版本之间仍可能变动。

## 为什么是 ezboot

- **不只是一个服务，而是一套生命周期。** 启动是有阶段、有预算、可回滚的状态机：某个插件始终不就绪会让启动失败，而不是带着半个服务接流量；排空开始时 `/readyz` 会先变成 503，再等在途请求结束。
- **装配是显式的。** 没有全局容器。依赖在编译期由 [wire](https://github.com/google/wire) 装配，关闭顺序就是你手写列表的逆序——一个文件里看得见，而不是藏在框架内部。
- **能力齐全，但按需引入。** HTTP、带网关的 gRPC、六个基础设施插件、tracing、RBAC 与代码生成都在仓库里，但 import 框架本身不会替你打开一条 Redis 连接。你注册你真正要跑的插件。

全部契约就是五个方法：

```go
type Plugin interface {
	Name() string
	Start(ctx context.Context) error // 阻塞到插件停止
	Ready(ctx context.Context) error // “这个实例能接流量吗”——只在启动期调用
	Drain(ctx context.Context) error // 停止接收新工作，等在途工作结束
	Close(ctx context.Context) error // 释放资源
}
```

嵌入 `PluginBase` 即可继承默认实现（`Start` 阻塞在 ctx 上，其余为空操作），只覆盖你真正关心的阶段。

## 环境要求

- Go 1.27 或更高版本
- Docker，仅在执行代码生成目标（`make protobuf`、`make wire`）时需要

## 安装

```bash
go get github.com/ez4bk/ezboot

# 项目生成器
go install github.com/ez4bk/ezboot/cmd/ezboot@latest
```

## 快速开始

从 [ezboot-template](https://github.com/ez4bk/ezboot-template) 生成一个服务：

```bash
ezboot new demo
cd demo
```

命令会先询问 Go module 前缀（例如 `github.com/you`），然后写进所有 import 路径与 `go.work`。生成的示例默认启用 MySQL 与 Redis 插件，所以先把它们跑起来（地址与凭据见 `config/local.yml`）：

```bash
docker run -d --name demo-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=demo \
  -e MYSQL_USER=user -e MYSQL_PASSWORD=password mysql:8
docker run -d --name demo-redis -p 6379:6379 redis:7
```

启动服务，然后检查就绪状态：

```bash
go run ./cmd/demo
curl -s localhost:8899/readyz   # {"status":"OK"}
```

在全部插件的 `Ready` 通过之前，`/readyz` 返回 503；开始排空的瞬间又会变回 503。`/_healthy` 只是存活探针，由进程自己回答，从不经过插件。

## 核心概念

| 概念 | 说明 |
|---|---|
| `Plugin` | 有生命周期的组件：`Start`、`Ready`、`Drain`、`Close`、`Name`。 |
| `PluginBase` | 五个方法的默认实现：`Start` 阻塞在 ctx 上，其余为空操作。 |
| `State` | 两个原子标志（`started`、`draining`），由生命周期写入、由 `/readyz` 读取。运行期的就绪判断不会去探测插件。 |
| `Application` | `ezboot.NewApp(...)` 加若干选项（`WithPlugins`、`WithState`、`WithLifecycleConfig`、`WithCmdline`）。`Start(ctx)` 直到排空与释放都完成才返回。 |
| `ProviderSet` | 给“每个服务都要、且没人会定制”的依赖准备的 wire 集合。 |
| `Lifecycle` | 驱动插件的内部状态机，不导出；你通过 `Application` 与 `State` 观察它。 |

## 用 wire 装配

ezboot 假设你使用 [google/wire](https://github.com/google/wire) 做依赖注入，这个假设是设计的一部分，而不是实现细节：

- **依赖图在编译期确定。** 缺 provider 是编译错误，不是运行时 panic，也没有 service locator 可以兜底。
- **构造顺序即依赖顺序。** tracer provider 必须先于 gRPC server 构造（拦截器在 driver 构造时被读取），所以把它写成构造参数，才真正保证了顺序。
- **没有自动注册。** 一个插件会运行，是因为你把它放进了列表，而不是某个 blank import 顺手做了事。

`ezboot.ProviderSet` 只包含通用且不可妥协的部分：

```go
var ProviderSet = wire.NewSet(
	ProvideConfig,        // 配置 + telemetry 初始化
	ProvideLogger,
	NewState,
	tracing.NewOTEL,
	grpcclient.NewGRPCClient,
)
```

HTTP、gRPC 与六个基础设施插件被刻意排除在外。它们需要应用特有的输入（路由、服务注册、网关 handler），而 wire 不支持同类型多绑定：把 `grpc.New` 放进公共集合会与应用自己的构造器冲突。

`[]ezboot.Plugin` 必须由一个函数收口，它的参数顺序就是关闭顺序：

```go
func NewPlugins(
	otel *tracing.OTEL,
	db *infra.Databases,
	gc *grpcclient.GRPCClient,
	gs *grpc.Server,
	hs *http.Server,
) []ezboot.Plugin {
	// 逆序关闭：http → grpc → grpc-client → db → otel
	return []ezboot.Plugin{otel, db, gc, gs, hs}
}
```

因此“加一个插件”等于改这个签名并重新生成（`wire ./...`；模板把它包在 `make wire` 里）。这份摩擦是刻意的：它让组合根在 diff 里可评审。

## 生命周期

```
Start（并发）
  → 轮询 Ready          直到全部通过，或 ready_timeout 耗尽
  → started
  → 等待退出信号
  → draining            /readyz 开始返回 503
  → 逆序 Drain（串行）    停止接收新工作，等在途工作结束
  → cancel Start ctx
  → 逆序 Close（串行）    释放资源
```

- **Start 并发执行**，启动耗时等于最慢的插件，而不是所有插件之和。某个 `Start` 在退出流程开始前就返回，等同故障，会拆掉整个应用。
- **Ready 被轮询**，每 100 ms 一次、单次调用上限 2 s，直到全部插件通过或 `ready_timeout` 耗尽。启动失败会回滚：取消其余插件，并关闭所有已经启动的插件。
- **先 Drain 再 Close**，都按注册逆序、串行执行，各自拿到新建的、带超时的 ctx。两者不可互换：`Drain` 停止接收新工作并等在途工作结束，`Close` 释放资源。
- **顺序有意义。** 注册顺序**就是**关闭顺序：基础设施在前（最后关闭），入口在后（最先关闭），OpenTelemetry 放在最前，好让它在其他插件都停下之后才冲刷 span。

各阶段的预算配置在 `app.lifecycle` 下，零值回落到下面显示的默认值：

```yaml
app:
  lifecycle:
    ready_timeout: 10s   # 每个插件，启动期
    drain_timeout: 10s   # 每个插件
    close_timeout: 5s    # 每个插件
```

在 Kubernetes 里，`terminationGracePeriodSeconds` 要大于 drain 与 close 预算之和，并留出余量。

## 插件

| 插件 | 包 | 职责 | `Ready` 的含义 |
|---|---|---|---|
| HTTP | `http` | gin 引擎、中间件、pprof、可选 `/metrics`、`/_healthy`、`/readyz` | 端口已绑定 |
| gRPC | `grpc` | 服务、网关 handler、reflection、Prometheus、优雅停止 | 端口已绑定且网关就绪 |
| gRPC 客户端池 | `grpc/client` | 进程级客户端连接，首次使用时建立 | 始终就绪（连接是懒加载的） |
| OpenTelemetry | `tracing` | tracer provider、拦截、关闭时冲刷 span | 始终就绪 |
| 数据库 | `infra` | 通过 xorm 支持 MySQL、SQLite 或 PostgreSQL——每个进程只允许一种方言 | 所有已配置连接池 ping 通过 |
| Redis | `infra` | 按命名实例提供客户端 | ping |
| Kafka | `infra` | broker，以及按 topic 创建的 reader/writer | broker 可达 |
| RabbitMQ | `infra` | 带重连循环的连接 | channel 可用 |
| Elasticsearch | `infra` | 搜索客户端 | ping |
| Etcd | `infra` | client v3 | endpoint 可达 |

每个基础设施插件都自带 `ProviderSet` 与 default provider，因此业务代码依赖的是原生类型（`*xorm.Engine`、`*redis.Client`），而不是框架再包一层。

自己写插件同样是那五个方法：嵌入 `PluginBase`，实现 `Name` 和你需要的阶段，然后加进 `NewPlugins`。构造函数可以创建客户端对象，但不得做网络 I/O、不得绑定端口——那属于 `Start`；而“依赖是否真的可用”由 `Ready` 证明。模板里在 `internal/plugin/example` 下有一个完整示例。

## 周边组件

| 包 | 用途 |
|---|---|
| `rbac` | 基于 Casbin 的 RBAC 模型、策略定义与加载 |
| `exorm` | xorm 查询辅助：条件、分页、游标分页 |
| `bizerr` | 业务错误类型，带 gRPC 状态与网关映射 |
| `snowflake` | Snowflake ID 生成 |
| `retry` | 基于 backoff 的指数与固定间隔重试 |
| `cache` | 面向可二进制序列化值的 Redis 缓存 |
| `flight` | 泛型 singleflight 请求合并 |
| `exp` | 泛型工具：lazy 值、map、取值、AES/RSA |
| `xjson` | JSON 辅助 |
| `xref` | 反射辅助 |
| `elog` | 记录日志并包装错误的辅助 |
| `utils` | 指针与字符串工具 |
| `signal` | 通过 `signal.WithExit` 获得 `SIGINT`/`SIGTERM` 上下文 |
| `with` | 取用插件注册的默认 engine 与 Redis 句柄 |

## 工具链

| 工具 | 用途 |
|---|---|
| `ezboot new` | 从模板仓库生成服务 |
| `ezboot model` | 从已有数据库结构生成 model 定义 |
| `protoc-gen-go-ezboot` | ezboot 服务胶水代码的 protobuf 生成器 |
| `protoc-gen-go-rbac` | RBAC 定义的 protobuf 生成器 |
| `make protobuf` | 在固定容器镜像里执行 `buf generate` |
| `tools/reflect` | 从 model 结构体生成 DAO |
| `cmd/snowflake` | 独立的 Snowflake ID 生成器 |

## 文档

- [Wiki](https://github.com/ez4bk/ezboot/wiki) —— 技术细节（插件、生命周期、装配、配置）与架构变迁记录
- [ezboot-template](https://github.com/ez4bk/ezboot-template) —— 参考服务：wire 装配、自定义插件、Helm chart
