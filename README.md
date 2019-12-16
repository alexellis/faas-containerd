# faas-containerd

[OpenFaaS](https://github.com/openfaas/faas) provider for containerd - single node / edge workloads

What's the use-case?

OpenFaaS providers can be built for any backend, even for an in-memory datastore. Some users could benefit from a lightweight, single-node execution environment. Using containerd and bypassing Kubernetes or Docker should reduce the start-time for functions and allow for running in resource-constrained environments.

## Status

Proof of concept.

This project implements the [faas-provider](https://github.com/openfaas/faas-provider) SDK.

See other examples:

* [faas-memory](https://github.com/openfaas-incubator/faas-memory/)
* [faas-swarm](https://github.com/openfaas/faas-swarm/)
* [faas-netes](https://github.com/openfaas/faas-netes/)

Goals:

- [x] Deploy container specified via `PUT` to `/system/functions`
- [ ] Retrieve logs from container
- [ ] Serve HTTP traffic from deployed container
- [ ] List running containers
- [ ] Clean-up containers on exit

## Test it out

Get and [start containerd](https://github.com/containerd/containerd) on a Linux computer, or VM.

```sh
sudo containerd
```

Install Go:

```sh
curl -SLsf https://dl.google.com/go/go1.12.14.linux-amd64.tar.gz > go.tgz
sudo rm -rf /usr/local/go/
sudo mkdir -p /usr/local/go/
sudo tar -xvf go.tgz -C /usr/local/go/ --strip-components=1

export GOPATH=$HOME/go/
export PATH=$PATH:/usr/local/go/bin/

go version
```

Get netns

```sh
go get -u github.com/genuinetools/netns
```

> Make sure "netns" is in $PATH

Create [networking configuration for CNI](https://github.com/containernetworking/cni/tree/master/cnitool)

```sh
echo '{"cniVersion":"0.4.0","name":"myptp","type":"ptp","ipMasq":true,"ipam":{"type":"host-local","subnet":"172.16.29.0/24","routes":[{"dst":"0.0.0.0/0"}]}}' | sudo tee /etc/cni/net.d/10-myptp.conf
```

Build and run

```sh
git clone https://github.com/alexellis/faas-containerd
cd faas-containerd
go build

sudo ./faas-containerd
```

> Listens on port TCP/8081

Deploy a container without a server

```sh
curl -d '{"service":"uptime", "image":"alexellis2/uptime:latest" }' \
  -X PUT http://127.0.0.1:8081/system/functions
```

Deploy a function with a server

```sh
curl -d '{"service":"nodeinfo","image":"functions/nodeinfo","envProcess":"node main.js"}' \
  -X PUT http://127.0.0.1:8081/system/functions
```

List containers:

```sh
sudo ctr list --namespace openfaas-fn
```

## License

MIT

