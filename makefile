NAME := cfirewall-func
TAG ?= 1.0
REGISTRY=docker.io/wang5150753
IMAGE := $(REGISTRY)/$(NAME):$(TAG)


all:
	- docker rmi $(IMAGE)
	docker build -f Dockerfile -t $(IMAGE) .
	# docker save -o $(NAME).tar $(IMAGE)
	docker image prune -f
	docker push $(IMAGE)

.PHONY: all
