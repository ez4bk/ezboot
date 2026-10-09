override LATEST_TAG := 3
override IMAGE_NAME := ghcr.io/ez4bk/devops/alpine

.DEFAULT_GOAL: all
.PHONY: 3
all: 3 latest

3: Dockerfile
	docker build --squash -f $^ -t $(IMAGE_NAME):3 .
	docker push $(IMAGE_NAME):3

latest:
	docker build --squash -f $(LATEST_TAG)/Dockerfile -t $(IMAGE_NAME) .
	docker push $(IMAGE_NAME)
