// Package infra 把外部依赖封装成插件：数据库、Redis、消息队列、搜索
// 和 etcd。
//
// 每个插件负责打开、暴露和关闭自己那一类客户端，并实现 ezboot.Plugin
// 的 Start / Ready / Close。插件之间互不感知，装配顺序由调用方的 wire
// 与 []ezboot.Plugin 决定。
//
// 这个包不感知 HTTP / gRPC 的请求管道；唯一的 middleware 依赖是
// middleware/tracing/otelxorm 里的追踪钩子。
package infra
