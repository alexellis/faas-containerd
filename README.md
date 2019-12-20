# faas-containerd

[OpenFaaS](https://github.com/openfaas/faas) provider for containerd - single node / edge workloads

What's the use-case?

OpenFaaS providers can be built for any backend, even for an in-memory datastore. Some users could benefit from a lightweight, single-node execution environment. Using containerd and bypassing Kubernetes or Docker should reduce the start-time for functions and allow for running in resource-constrained environments.

Pros:
* Fast cold-start
* containerd features available such as pause/snapshot
* Super lightweight

Cons:
* No clustering (yet)
* No inter-service communication (yet)

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
- [x] Serve HTTP traffic from deployed container via `/function/NAME`
- [x] List running containers via GET on `/system/functions`
- [ ] Clean-up containers on exit
- [ ] Give configuration for running faas-containerd / OpenFaaS gateway and Prometheus via systemd unit files or similar

## Demo

![](https://pbs.twimg.com/media/EMEg1OEWkAAIDPO?format=jpg&name=medium)

* [View the Tweet](https://twitter.com/alexellisuk/status/1207282296459595776)

## Test it out

You need a Linux computer, VM, or bare-metal cloud host.

I used Ubuntu 18.04 LTS on [Packet.com using the c1.small.x86](https://www.packet.com/cloud/servers/c1-small/) host. You can use [multipass.run](https://multipass.run) to get an Ubuntu host on any OS - Windows, MacOS, or Linux.

Install [containerd](https://github.com/containerd/containerd):

```
sudo apt update && \
  sudo apt install -qy containerd golang runc bridge-utils ethtool tmux git
```

Check containerd started:

```sh
systemctl status containerd
```

Enable forwarding:

```sh
/sbin/sysctl -w net.ipv4.conf.all.forwarding=1
```

Get netns

```sh
export GOPATH=$HOME/go/

go get -u github.com/genuinetools/netns
sudo mv $GOPATH/bin/netns /usr/bin/
```

Build and run

```sh
export GOPATH=$HOME/go/

mkdir -p $GOPATH/src/github.com/alexellis/faas-containerd
cd $GOPATH/src/github.com/alexellis/faas-containerd
git clone https://github.com/alexellis/faas-containerd
cd faas-containerd
go build && sudo function_uptime=120m ./faas-containerd
```

> Listens on port TCP/8081

Get the OpenFaaS CLI:

```sh
curl -sLfS https://cli.openfaas.com | sudo sh
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

Try to list functions:

```sh
faas-cli list -g 127.0.0.1:8081
```

Get a function's status:
```sh
faas-cli describe nodeinfo -g 127.0.0.1:8081
```

Try to invoke a function:

```sh
echo "-c 1 8.8.8.8" | faas-cli invoke ping -g 127.0.0.1:8081

echo "verbose" | faas-cli invoke nodeinfo -g 127.0.0.1:8081
```

List containers with `ctr`:

```sh
sudo ctr --namespace openfaas-fn containers list
```

Delete containers or snapshots:

```sh
sudo ctr --namespace openfaas-fn snapshot delete figlet
sudo ctr --namespace openfaas-fn snapshot delete figlet-snapshot
```

* Appendix


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

Deploy a container without a server

```sh
faas deploy --name uptime --image alexellis2/uptime:latest \
  -g 127.0.0.1:8081 --update=true --replace=false
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

## Links

* [Detailed explanation on netns](https://pierrchen.blogspot.com/2018/05/understand-container-6-hooks-and-network.html)

## License

MIT
