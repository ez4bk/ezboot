module github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac

go 1.27

require (
	github.com/ez4bk/ezboot v0.2.0
	google.golang.org/genproto/googleapis/api v0.0.0-20251213004720-97cd9d5aeac2
	google.golang.org/protobuf v1.36.11
)

replace github.com/ez4bk/ezboot => ../..

require (
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/casbin/casbin/v2 v2.135.0 // indirect
	github.com/casbin/govaluate v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
)
