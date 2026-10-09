override PLUGIN_NAME := protoc-gen-ezboot

.DEFAULT_GOAL := generate
.PHONY: protoc-gen-ezboot generate
.DEFAULT_GOAL: generate


protoc-gen-ezboot: main.go
	CGO_ENABLED=0 go install

generate: protoc-gen-ezboot testdata/simple.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--go-ezboot_out=. --go-ezboot_opt=paths=source_relative \
		testdata/simple.proto

install-deps:
	brew install protobuf
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
