# Binary name
BINARY_NAME ?= spiffe-client

# Container tool to be used for building images
CONTAINER_TOOL ?= docker

# Image tag
TAG ?= latest

# Platform constants
PLATFORM ?= linux/arm64

# Remote (Docker Hub)
DOCKER_REGISTRY ?= semenyukandriyv
IMG ?= $(DOCKER_REGISTRY)/$(BINARY_NAME):$(TAG)

# # Development builds (with debug info)
# build-linux:
# 	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux .

# build-darwin:
# 	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin .

# build-windows:
# 	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows.exe .

# # Production builds (optimized, stripped)
# build-linux-prod:
# 	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY_NAME)-linux-prod .

# build-darwin-prod:
# 	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY_NAME)-darwin-prod .

# build-windows-prod:
# 	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY_NAME)-windows-prod.exe .

# clean:
# 	rm -f $(BINARY_NAME)-*
# 	rm -f $(BINARY_NAME).exe

# # Build all platforms (development)
# build-all:
# 	$(MAKE) build-linux
# 	$(MAKE) build-darwin
# 	$(MAKE) build-windows

# # Build all platforms (production)
# build-all-prod:
# 	$(MAKE) build-linux-prod
# 	$(MAKE) build-darwin-prod
# 	$(MAKE) build-windows-prod

# Normal build docker image (uses cache)
docker-build:
	DOCKER_BUILDKIT=1 $(CONTAINER_TOOL) build \
		--platform $(PLATFORM) \
		-t $(IMG) .

# Clean build docker image (no cache)
docker-build-no-cache:
	DOCKER_BUILDKIT=1 $(CONTAINER_TOOL) build \
		--no-cache \
		--platform $(PLATFORM) \
		-t $(IMG) .

# Push docker image to remote registry
docker-push-remote:
	DOCKER_BUILDKIT=1 $(CONTAINER_TOOL) push $(IMG)

# Remove local docker image
docker-clean-local:
	$(CONTAINER_TOOL) rmi -f $(IMG) || true

# Deploy to cluster
deploy:
	kubectl apply -f spiffe-client-ns.yaml
	kubectl apply -f spiffe-client-sa.yaml
	kubectl apply -f spiffe-client.yaml
	kubectl apply -f spiffe-client-deployment.yaml