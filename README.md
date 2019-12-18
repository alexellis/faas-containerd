# faas-containerd

[OpenFaaS](https://github.com/openfaas/faas) provider for containerd - single node / edge workloads

What's the use-case?

OpenFaaS providers can be built for any backend, even for an in-memory datastore. Some users could benefit from a lightweight, single-node execution environment. Using containerd and bypassing Kubernetes or Docker should reduce the start-time for functions and allow for running in resource-constrained environments.

## Status

Proof of concept.

This project implements the [faas-provider](https://github.com/openfaas/faas-provider) SDK.

![Architecture](https://github.com/openfaas/faas-provider/raw/master/docs/conceptual.png)

*faas-provider conceptual architecture*

See other examples:

* [faas-memory](https://github.com/openfaas-incubator/faas-memory/)
* [faas-swarm](https://github.com/openfaas/faas-swarm/)
* [faas-netes](https://github.com/openfaas/faas-netes/)

Goals:

- [x] Deploy container specified via `PUT` to `/system/functions`
- [ ] Serve HTTP traffic from deployed container via `/function/NAME`
- [ ] List running containers via GET on `/system/functions`
- [ ] Clean-up containers on exit
- [ ] Give configuration for running faas-containerd / OpenFaaS gateway and Prometheus via systemd unit files or similar

## Test it out

Install [containerd](https://github.com/containerd/containerd) on a Linux computer, or VM:

```
sudo apt update && sudo apt install -qy containerd golang runc bridge-utils ethtool
```

Check containerd started:
```sh
systemctl status containerd
```

Enable forwarding:

```sh
/sbin/sysctl -w net.ipv4.conf.all.forwarding=1
```

Install Go if a newer version is required (optional)

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
export GOPATH=$HOME/go/

go get -u github.com/genuinetools/netns
sudo mv $GOPATH/bin/netns /usr/bin/
```

Create [networking configuration for CNI](https://github.com/containernetworking/cni/tree/master/cnitool)

```sh
$ mkdir -p /etc/cni/net.d
$ cat >/etc/cni/net.d/10-mynet.conf <<EOF
{
	"cniVersion": "0.2.0",
	"name": "mynet",
	"type": "bridge",
	"bridge": "cni0",
	"isGateway": true,
	"ipMasq": true,
	"ipam": {
		"type": "host-local",
		"subnet": "10.22.0.0/16",
		"routes": [
			{ "dst": "0.0.0.0/0" }
		]
	}
}
EOF
```

Build and run

```sh
git clone https://github.com/alexellis/faas-containerd
cd faas-containerd
go build && sudo ./faas-containerd
```

> Listens on port TCP/8081

Deploy a container without a server


```sh
faas deploy --name uptime --image alexellis2/uptime:latest \
  -g 127.0.0.1:8081 --update=true --replace=false
```

Deploy a function with a server

```sh
faas store deploy figlet -g 127.0.0.1:8081 --update=true --replace=false
```

Deploy a ping function with a server

```sh
faas-cli deploy --image alexellis2/ping:0.1 \
  -g 127.0.0.1:8081 --update=true --replace=false --name ping
```

Deploy nodeinfo function with a server

```sh
faas-cli store deploy nodeinfo \
  -g 127.0.0.1:8081 --update=true --replace=false
```

List containers:

```sh
sudo ctr list --namespace openfaas-fn
```

Delete containers or snapshots:

```sh
sudo ctr --namespace openfaas-fn snapshot delete figlet
sudo ctr --namespace openfaas-fn snapshot delete figlet-snapshot
```

## Links

* [Detailed explanation on netns](https://pierrchen.blogspot.com/2018/05/understand-container-6-hooks-and-network.html)

## License

MIT

