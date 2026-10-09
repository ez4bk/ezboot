override PLUGIN_NAME := protoc-gen-rbac

.DEFAULT_GOAL := generate
.PHONY: protoc-gen-ezboot generate
.DEFAULT_GOAL: generate


protoc-gen-ezboot: main.go
	CGO_ENABLED=0 go install

generate: protoc-gen-ezboot testdata/rbac/simple.proto
	protoc -I testdata -I ../../ \
		--go_out=testdata --go_opt=paths=source_relative \
		--go-grpc_out=testdata --go-grpc_opt=paths=source_relative \
		--go-rbac_out=testdata --go-rbac_opt=paths=source_relative \
		testdata/rbac/simple.proto

install-deps:
	brew install protobuf
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
