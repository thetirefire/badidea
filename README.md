# badidea

Minimal embeddable Kubernetes-style apiserver that supports CustomResourceDefitions

## Prerequisites

- etcd binary
- kubectl binary

## Start the etcd server
```sh
etcd
```

## Start the badidea server
```sh
go run main.go
```

## Do the thing
```sh
# username and password are ignored, but required for the command to complete
kubectl --server https://localhost:6443 --insecure-skip-tls-verify --username=bad --password=idea <the thing>
```