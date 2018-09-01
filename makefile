VERSION := $(shell git describe --tags)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
VAULT_TOKEN := "fdb46bbe-fd53-7a71-b3e9-c9c0683869dc"
-include .env

fast:
	go build \
		$(LDFLAGS) \
		-o machinehead

static:
	CGO_ENABLED=0 GOOS=linux \
	go build \
		-a $(LDFLAGS) \
		-o machinehead

local: fast
	DEBUG=1 \
	./machinehead


# -
# Docker
# -


build:
	docker build --no-cache -t southclaws/machinehead:$(VERSION) .

push:
	docker push southclaws/machinehead:$(VERSION)

run:
	-docker stop machinehead
	-docker rm machinehead
	docker run \
		--name machinehead \
		--network host \
		--env-file .env \
		southclaws/machinehead:$(VERSION)


# -
# Testing
# -


vault:
	-docker stop vault
	-docker rm vault
	docker run \
		--name=vault \
		--cap-add=IPC_LOCK \
		--detach \
		--publish 8200:8200 \
		-e VAULT_DEV_ROOT_TOKEN_ID=$(VAULT_TOKEN) \
		vault
