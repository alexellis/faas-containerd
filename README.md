# faas-containerd

OpenFaaS provider for containerd - single node / edge workloads

## Status

Proof of concept.

## Test it out

Get netns

```sh
go get -u github.com/genuinetools/netns
```

> Make sure "netns" is in $PATH

Build and run

```sh
git clone https://github.com/alexellis/faas-containerd
cd faas-containerd
go build

./faas-containerd
```

> Listens on port TCP/8081

Invoke a function

```sh
curl -d '{"service":"nodeinfo","image":"functions/nodeinfo:burner","envProcess":"node main.js","labels":{"com.openfaas.scale.min":"2","com.openfaas.scale.max":"15"},"environment":{"output":"verbose","debug":"true"}}' -X PUT http://127.0.0.1:8081/system/functions
```

## License

MIT

