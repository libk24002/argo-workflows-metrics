.PHONY: build run docker-build docker-push clean test fmt vet

BINARY_NAME=exporter
DOCKER_IMAGE=argo-workflows-metrics
DOCKER_TAG=latest

build:
	go build -o bin/$(BINARY_NAME) ./cmd/exporter

run:
	go run ./cmd/exporter

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

clean:
	rm -rf bin/
	go clean

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
