override IMAGE_NAME ?= ghcr.io/ez4bk/devops/protoc-buf
#override IMAGE_NAME ?= crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/ezboot.protoc-buf

.DEFAULT_GOAL: image
.PHONY: image
# TODO: 版本稳定后恢复 --push .
image: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -t $(IMAGE_NAME) --push --file Dockerfile ../..
