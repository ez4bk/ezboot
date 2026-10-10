FROM ghcr.io/ez4bk/devops/go-compiler AS builder

RUN go install github.com/bufbuild/buf/cmd/buf@latest
RUN go install github.com/favadi/protoc-go-inject-tag@latest
RUN go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
RUN go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
RUN go install github.com/envoyproxy/protoc-gen-validate@latest
RUN go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-ezboot@main
RUN go install github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac@main

# TODO: 版本稳定后恢复
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/protoc-gen-go-ezboot ./cmd/protoc-gen-go-ezboot
COPY cmd/protoc-gen-go-rbac ./cmd/protoc-gen-go-rbac
COPY rbac ./rbac
COPY utils ./utils
RUN cd cmd/protoc-gen-go-ezboot && go install
RUN cd cmd/protoc-gen-go-rbac && go install


FROM ubuntu:24.04

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["buf"]
