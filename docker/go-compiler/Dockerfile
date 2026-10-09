FROM golang:1.27-alpine

WORKDIR /workspace
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk add --no-cache make bash git openssh tzdata

ENV LANG=en_US.utf-8
ENV LC_ALL=en_US.utf-8

ENV GOPATH=/go
ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

#ADD ssh1.26 /root/.ssh
#RUN chmod -R g-rwx,o-rwx /root/.ssh

RUN git config --global --add safe.directory /workspace
