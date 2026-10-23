// Package middleware contains HTTP and gRPC request-pipeline middleware only:
// logging, recovery, validation, metadata helpers and tracing interceptors.
//
// Durable external dependencies such as databases, Redis and RabbitMQ belong
// to the infra package, not here. Middleware may use the request context and
// the tracing subpackages, but it must not own connection pools or clients.
package middleware
