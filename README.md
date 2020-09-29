<a href="https://godoc.org/github.com/thetirefire/badidea"><img src="https://godoc.org/github.com/thetirefire/badidea?status.svg"></a>

# badidea

Minimal embeddable Kubernetes-style apiserver that supports CustomResourceDefitions

## Prerequisites

- kubectl binary

## Development Prerequisites

- Go v1.15+

## Build the badidea server

```sh
make badidea
```

## Start the badidea server

```sh
bin/badidea
```

## Do the thing

```sh
# username and password are ignored, but required for the command to complete
kubectl --server https://localhost:6443 --insecure-skip-tls-verify --username=bad --password=idea <the thing>
```
