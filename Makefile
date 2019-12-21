.PHONY: all build push

TAG?=latest

all: build push

build:
	docker build -t alexellis2/faas-containerd:$(TAG) .

push:
	docker push alexellis2/faas-containerd:$(TAG)

