FROM ghcr.io/ez4bk/devops/go-compiler
#FROM crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.go-compiler

RUN go install github.com/google/wire/cmd/wire@latest && rm -rf $GOPATH/pkg/*
