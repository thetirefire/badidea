# badidea

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
kubectl --server https://localhost:6443 --insecure-skip-tls-verify --username=bad --password=idea <the thing>
```