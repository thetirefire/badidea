.PHONY: fix fmt vet lint test tidy

GOBIN := $(shell go env GOPATH)/bin

all: fix fmt vet lint test tidy badidea

docker:
	docker build ./ --tag badidea:latest

badidea:
	CGO_ENABLED=0 go build -o bin/badidea ./

fix:
	go fix ./...

fmt:
	test -z $(go fmt ./tools/...)

tidy:
	go mod tidy

lint:
	(which golangci-lint || go get github.com/golangci/golangci-lint/cmd/golangci-lint)
	$(GOBIN)/golangci-lint run ./...

test:
	go test -cover ./...

vet:
	go vet ./...

clean:
	rm -rf apiserver.local.config default.etcd bin/