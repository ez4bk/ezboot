module github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac/testdata

go 1.27

require (
	github.com/casbin/casbin/v3 v3.11.0
	github.com/ez4bk/ezboot v0.3.0
	google.golang.org/genproto/googleapis/api v0.0.0-20261005182115-fad411399dd8
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.3 // indirect
	github.com/casbin/govaluate v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261005182115-fad411399dd8 // indirect
)

replace github.com/ez4bk/ezboot => ../../..
