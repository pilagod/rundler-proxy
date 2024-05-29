SHELL:=/usr/bin/env bash

TAG = latest
.PHONY: build-docker-image
build-docker-image:
	docker buildx build . -t pilagod/rundler-proxy:$(TAG) --platform linux/amd64,linux/arm64 --builder container --push
