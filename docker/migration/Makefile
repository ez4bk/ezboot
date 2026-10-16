override IMAGE_NAME ?= ghcr.io/ez4bk/devops/migration
#override IMAGE_NAME ?= crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.migration

.DEFAULT_GOAL: image
.PHONY: image
image: Dockerfile
	docker build --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME) --push .
