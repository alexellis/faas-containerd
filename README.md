# faas-containerd

[![Build Status](https://travis-ci.com/alexellis/faas-containerd.svg?branch=master)](https://travis-ci.com/alexellis/faas-containerd)

[OpenFaaS](https://github.com/openfaas/faas) provider for containerd - single node / edge workloads

What's the use-case?

OpenFaaS providers can be built for any backend, even for an in-memory datastore. Some users could benefit from a lightweight, single-node execution environment. Using containerd and bypassing Kubernetes or Docker should reduce the start-time for functions and allow for running in resource-constrained environments.

Pros:
* Fast cold-start
* containerd features available such as pause/snapshot
* Super lightweight
* Basic service-discovery and inter-service communication through /etc/hosts and bridge

Cons:
* No clustering (yet)

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

### Get some build dependencies

I used Ubuntu 18.04 LTS on [Packet.com using the c1.small.x86](https://www.packet.com/cloud/servers/c1-small/) host. You can use [multipass.run](https://multipass.run) to get an Ubuntu host on any OS - Windows, MacOS, or Linux.

```sh
sudo apt update && \
  sudo apt install -qy runc \
  	bridge-utils \
	tmux git \
  	build-essential \
	libbtrfs-dev libseccomp-dev
```

### Install Go 1.12 (x86_64)

```sh
curl -SLsf https://dl.google.com/go/go1.12.14.linux-amd64.tar.gz > go.tgz
sudo rm -rf /usr/local/go/
sudo mkdir -p /usr/local/go/
sudo tar -xvf go.tgz -C /usr/local/go/ --strip-components=1

export GOPATH=$HOME/go/
export PATH=$PATH:/usr/local/go/bin/

go version
```

### Or on Raspberry Pi (armhf)

```sh
curl -SLsf https://dl.google.com/go/go1.12.14.linux-armv6l.tar.gz > go.tgz
sudo rm -rf /usr/local/go/
sudo mkdir -p /usr/local/go/
sudo tar -xvf go.tgz -C /usr/local/go/ --strip-components=1

export GOPATH=$HOME/go/
export PATH=$PATH:/usr/local/go/bin/

go version
```

### Get containerd

* Install containerd (or build from source)

> Note: This can only be run on x86_64

```sh
export VER=1.3.2
curl -sLSf https://github.com/containerd/containerd/releases/download/v$VER/containerd-$VER.linux-amd64.tar.gz > /tmp/containerd.tar.gz \
  && sudo tar -xvf /tmp/containerd.tar.gz -C /usr/local/bin/ --strip-components=1

containerd -version
```

* Or clone / build / install [containerd](https://github.com/containerd/containerd) from source:

```sh
export GOPATH=$HOME/go/
mkdir -p $GOPATH/src/github.com/containerd
cd $GOPATH/src/github.com/containerd
git clone https://github.com/containerd/containerd
cd containerd
git fetch origin --tags
git checkout v1.3.2

make
sudo make install

containerd --version
```

Kill any old containerd version:

```sh
# Kill any old version
sudo killall containerd
sudo systemctl disable containerd
```

Start containerd in a new terminal:

```sh
sudo containerd &
```

### Enable forwarding:

> This is required to allow containers in containerd to access the Internet via your computer's primary network interface.

```sh
sudo /sbin/sysctl -w net.ipv4.conf.all.forwarding=1
```

Make the setting permanent:

```
echo "net.ipv4.conf.all.forwarding=1" | sudo tee -a /etc/sysctl.conf
```

### Get netns

* From binaries:

	```sh
	# For x86_64
	sudo curl -fSLs "https://github.com/genuinetools/netns/releases/download/v0.5.3/netns-linux-amd64" \
	  -o "/usr/local/bin/netns" \
	  && sudo chmod a+x "/usr/local/bin/netns"

	# armhf
	sudo curl -fSLs "https://github.com/genuinetools/netns/releases/download/v0.5.3/netns-linux-arm" \
	  -o "/usr/local/bin/netns" \
	  && sudo chmod a+x "/usr/local/bin/netns"

	# arm64
	sudo curl -fSLs "https://github.com/genuinetools/netns/releases/download/v0.5.3/netns-linux-arm64" \
	  -o "/usr/local/bin/netns" \
	  && sudo chmod a+x "/usr/local/bin/netns"
	```

* Or build from source:

	```sh
	export GOPATH=$HOME/go/

	go get -u github.com/genuinetools/netns
	sudo mv $GOPATH/bin/netns /usr/bin/
	```

### Build and run faas-containerd

* Get a binary

	```sh
	# For x86_64
	sudo curl -fSLs "https://github.com/alexellis/faas-containerd/releases/download/0.3.3/faas-containerd" \
	  -o "/usr/local/bin/faas-containerd" \
	  && sudo chmod a+x "/usr/local/bin/faas-containerd"

	# armhf
	sudo curl -fSLs "https://github.com/alexellis/faas-containerd/releases/download/0.3.3/faas-containerd-armhf" \
	  -o "/usr/local/bin/faas-containerd" \
	  && sudo chmod a+x "/usr/local/bin/faas-containerd"

	# arm64
	sudo curl -fSLs "https://github.com/alexellis/faas-containerd/releases/download/0.3.3/faas-containerd-arm64" \
	  -o "/usr/local/bin/faas-containerd" \
	  && sudo chmod a+x "/usr/local/bin/faas-containerd"
	  
	  sudo service_timeout=1m ./faas-containerd
	```

* Or build from source

	```sh
	export GOPATH=$HOME/go/

	mkdir -p $GOPATH/src/github.com/alexellis/faas-containerd
	cd $GOPATH/src/github.com/alexellis/faas-containerd
	git clone https://github.com/alexellis/faas-containerd
	cd faas-containerd

	go build && sudo service_timeout=1m ./faas-containerd
	```

> Listens on port TCP/8081

## Test out your faas-containerd

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
sudo ctr --namespace openfaas-fn container list
```

Delete container, snapshot and task:

```sh
sudo ctr --namespace openfaas-fn task kill figlet
sudo ctr --namespace openfaas-fn task delete figlet
sudo ctr --namespace openfaas-fn container delete figlet
sudo ctr --namespace openfaas-fn snapshot remove figlet-snapshot
```

## Links

* [Detailed explanation on netns](https://pierrchen.blogspot.com/2018/05/understand-container-6-hooks-and-network.html)

## License

MIT
