.DEFAULT_GOAL: image
.PHONY: image
# TODO: 版本稳定后恢复 --push .
image: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -t ghcr.io/ez4bk/devops/protoc-buf --push --file Dockerfile ../..
