NAME := cfirewall-func
TAG ?= 1.0
REGISTRY=registry.local:9001
IMAGE := $(REGISTRY)/$(NAME):$(TAG)


all:
	- docker rmi $(IMAGE)
	docker build -f Dockerfile -t $(IMAGE) .
	docker save -o $(NAME).tar $(IMAGE)
	docker image prune -f

.PHONY: all
