module github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac/testdata

go 1.27

require (
	github.com/casbin/casbin/v3 v3.11.0
	github.com/ez4bk/ezboot v0.2.5
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace github.com/ez4bk/ezboot => ../../..
