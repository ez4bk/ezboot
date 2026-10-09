override BUILD_ARGS ?=

.DEFAULT_GOAL: image
.PHONY: image
image: latest

.PHONY: latest
latest: 1.27
	docker tag ghcr.io/ez4bk/devops/go-compiler:$^ ghcr.io/ez4bk/devops/go-compiler:$@
	docker push ghcr.io/ez4bk/devops/go-compiler:$@

.PHONY: 1.27
1.27: Dockerfile
	docker buildx build --platform linux/amd64,linux/arm64 --pull --squash -t ghcr.io/ez4bk/devops/go-compiler:$@ -f $< $(BUILD_ARGS) --push .
