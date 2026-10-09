FROM alpine:3

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk add --no-cache tzdata

ENV TZ=Asia/Shanghai

