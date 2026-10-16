FROM ghcr.io/ez4bk/devops/go-compiler AS builder
#FROM crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.go-compiler AS builder

RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install github.com/favadi/protoc-go-inject-tag@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest && \
    go install github.com/envoyproxy/protoc-gen-validate@latest && \
    go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-ezboot@main && \
    go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac@main


FROM ubuntu:24.04

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["buf"]
