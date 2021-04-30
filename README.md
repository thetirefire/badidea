<p>
<a href="https://godoc.org/github.com/thetirefire/badidea"><img src="https://godoc.org/github.com/thetirefire/badidea?status.svg"></a>
<a href="https://pkg.go.dev/thetirefire/badidea"><img src="https://pkg.go.dev/badge/thetirefire/badidea" alt="PkgGoDev"></a>
<a href="https://goreportcard.com/report/github.com/thetirefire/badidea"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/thetirefire/badidea" /></a>
</p>

# badidea

Minimal embeddable Kubernetes-style apiserver that supports CustomResourceDefitions

[Presentation/Demo for SIG API Machinery on October 7, 2020](https://www.youtube.com/watch?v=n1L5a09wWas)

[Slide deck](https://docs.google.com/presentation/d/1TfCrsBEgvyOQ1MGC7jBKTvyaelAYCZzl3udRjPlVmWg/edit?usp=sharing)

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

## Troubleshoot

### `root.go:41] listen unix etcd-socket:2379: bind: address already in use`

`rm etcd-socket\:2379` fixes this.
