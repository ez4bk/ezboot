// Package infra owns the process-level external dependencies (databases,
// Redis, message brokers, search and etcd).
//
// A Set is responsible for opening, exposing and closing those clients. The
// package must not import the transport or request-pipeline middleware; the
// only allowed middleware dependency is the tracing instrumentation hook
// under middleware/tracing/otelxorm. The composition root (internal/gin-boot)
// is the only place that knows about both infra and middleware.
package infra
