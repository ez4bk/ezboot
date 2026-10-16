override IMAGE_NAME ?= ghcr.io/ez4bk/devops/google-wire
#override IMAGE_NAME ?= crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.google-wire

.DEFAULT_GOAL: image
.PHONY: image
image: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -t $(IMAGE_NAME) --push .
