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

conversion-gen:
	(which conversion-gen || go get k8s.io/code-generator/cmd/conversion-gen)
	conversion-gen -i=./apis/core/v1 -h hack/boilerplate.go.txt -O zz_generated.conversion
	conversion-gen -i=./controllers/namespace/config/v1alpha1 -h hack/boilerplate.go.txt -O zz_generated.conversion

deepcopy-gen:
	(which deepcopy-gen || go get k8s.io/code-generator/cmd/deepcopy-gen)
	deepcopy-gen -i=./apis/core,./apis/core/v1 -h hack/boilerplate.go.txt -O zz_generated.deepcopy
	deepcopy-gen -i=./controllers/namespace/config,./controllers/namespace/config/v1alpha1 -h hack/boilerplate.go.txt -O zz_generated.deepcopy

defaulter-gen:
	(which defaulter-gen || go get k8s.io/code-generator/cmd/defaulter-gen)
	defaulter-gen -i=./apis/core/v1 -h hack/boilerplate.go.txt -O zz_generated.defaults

openapi-gen:
	(which openapi-gen || go get k8s.io/code-generator/cmd/openapi-gen)
	openapi-gen -i=./apis/core/v1,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version -h hack/boilerplate.go.txt -O zz_generated.openapi -p generated/openapi

clean:
	rm -rf apiserver.local.config default.etcd bin/