.DEFAULT_GOAL: image
.PHONY: image
image: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -f Dockerfile -t ghcr.io/ez4bk/devops/ezboot --push ../..
