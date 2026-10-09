include ../../build/workflow/env.mk

.DEFAULT_GOAL: image
.PHONY: image
image: Dockerfile
	docker build --platform linux/amd64 -t ghcr.io/ez4bk/devops/migration --push .
