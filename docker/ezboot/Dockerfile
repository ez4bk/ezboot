FROM ghcr.io/ez4bk/devops/go-compiler AS builder

RUN go install github.com/ez4bk/ezboot/cmd/ezboot@latest


FROM ubuntu:24.04

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["ezboot"]
