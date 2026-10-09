.DEFAULT_GOAL: image
.PHONY: image
image: Dockerfile
	docker buildx build --platform linux/amd64 --pull --squash -t ghcr.io/ez4bk/devops/ezboot --push .
