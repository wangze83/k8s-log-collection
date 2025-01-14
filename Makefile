TARGET = start3-test/logconfig-sidecar
VERSION ?= 1.0
REGISTRY ?=

all: injector

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
docker-build:
	docker build \
			-t $(REGISTRY)/$(TARGET):latest \
			-t $(REGISTRY)/$(TARGET):$(VERSION) \
			--build-arg GOPROXY="https://goproxy.cn" \
			.

# Push the docker image
docker-push:
	docker push $(REGISTRY)/$(TARGET):latest
	docker push $(REGISTRY)/$(TARGET):$(VERSION)

restart: docker-build
	kind load docker-image $(REGISTRY)/$(TARGET):$(VERSION) --name test
	kubectl delete -f deploy/log/sidecar/deployment.yaml
	kubectl apply -f deploy/log/sidecar/deployment.yaml
