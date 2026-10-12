FROM ghcr.io/ez4bk/devops/go-compiler AS builder

WORKDIR /src
COPY . .

RUN cd cmd/ezboot && \
    go mod edit -replace github.com/ez4bk/ezboot=../.. && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ezboot .


FROM ubuntu:24.04

COPY --from=builder /out/ezboot /usr/local/bin/ezboot
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["ezboot"]
