FROM ghcr.io/ez4bk/devops/go-compiler AS builder

RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM ubuntu:jammy

COPY --from=builder /go/bin/* /usr/local/bin/
RUN apt update -y && apt install -y ca-certificates

ENTRYPOINT ["migrate"]
