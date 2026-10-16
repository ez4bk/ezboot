override BUILD_ARGS ?=
override IMAGE_NAME := ghcr.io/ez4bk/devops/go-compiler
#override IMAGE_NAME := crpi-r14rq0u598dowwcs.cn-hangzhou.personal.cr.aliyuncs.com/ez4bk/devops.go-compiler
.DEFAULT_GOAL: image
.PHONY: image
image: latest

.PHONY: latest
latest: 1.27
	docker tag $(IMAGE_NAME):$^ $(IMAGE_NAME):$@
	docker push $(IMAGE_NAME):$@

.PHONY: 1.27
1.27: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -t $(IMAGE_NAME):$@ -f $< $(BUILD_ARGS) --push .
