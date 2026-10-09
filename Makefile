override DOCKER_TTY ?= -t
override DOCKER_REGISTRY ?= ghcr.io/ez4bk/devops
override RT_PROTOBUF_GENERATOR := ghcr.io/ez4bk/devops/protoc-buf

override BUF_CACHE := $(HOME)/.cache/buf

.PHONY: protobuf
.DEFAULT_GOAL: protobuf

protobuf:
	@docker pull $(RT_PROTOBUF_GENERATOR)
	@$(eval VOLUME_NAME=$(shell bash -c 'openssl rand -hex 6'))
	@docker volume create $(VOLUME_NAME)
	@docker container create --name ${VOLUME_NAME} -v ${VOLUME_NAME}:/data busybox
	@docker cp ./internal ${VOLUME_NAME}:/data/internal
	@docker cp ./rbac ${VOLUME_NAME}:/data/rbac
	@docker cp ./buf.gen.yaml ${VOLUME_NAME}:/data
	@docker cp ./buf.yaml ${VOLUME_NAME}:/data
	@docker cp ./buf.lock ${VOLUME_NAME}:/data
	docker run -i $(DOCKER_TTY) -v $(VOLUME_NAME):/data -v $(BUF_CACHE):/root/.cache/buf --rm --entrypoint="/bin/bash" \
		$(RT_PROTOBUF_GENERATOR) -ec 'cd /data && buf generate'
	@docker cp ${VOLUME_NAME}:/data/internal .
	@docker cp ${VOLUME_NAME}:/data/rbac .
	@docker rm ${VOLUME_NAME}
	@docker volume rm ${VOLUME_NAME}
