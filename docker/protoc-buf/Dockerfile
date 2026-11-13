FROM ghcr.io/ez4bk/devops/go-compiler AS builder
#FROM crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.go-compiler AS builder

# The grpc-gateway tools are versioned together with the runtime in go.mod
# (v2.33.0): generated *_grpc.pb.go / *.pb.gw.go are compiled against it, and
# openapiv3-merge must agree with protoc-gen-openapiv3's output shape.
RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install github.com/favadi/protoc-go-inject-tag@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.33.0 && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv3@v2.33.0 && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/openapiv3-merge@v2.33.0 && \
    go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-ezboot@main && \
    go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac@main


FROM ubuntu:24.04

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["buf"]
