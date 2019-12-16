# faas-containerd

OpenFaaS provider for containerd - single node / edge workloads

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

## Test it out

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

./faas-containerd
```

> Listens on port TCP/8081

Deploy a container without a server

```sh
curl -d '{"service":"uptime", "image":"alexellis2/uptime:latest" }' -X PUT http://127.0.0.1:8081/system/functions
```

Deploy a function with a server

```sh
curl -d '{"service":"nodeinfo","image":"functions/nodeinfo","envProcess":"node main.js","labels":{"com.openfaas.scale.min":"2","com.openfaas.scale.max":"15"},"environment":{"output":"verbose","debug":"true"}}' -X PUT http://127.0.0.1:8081/system/functions
```

## License

MIT

