FROM ghcr.io/ez4bk/devops/go-compiler AS builder
#FROM crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.go-compiler AS builder

RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM ubuntu:jammy

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["migrate"]
