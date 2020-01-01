package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/alexellis/faas-containerd/config"

	"github.com/alexellis/faas-containerd/handlers"
	"github.com/openfaas/faas-provider/proxy"

	"github.com/containerd/containerd"

	bootstrap "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/types"
)

var (
	Version   string
	GitCommit string
)

func main() {
	Start()
}

// Start faas-containerd
func Start() {

	config, providerConfig, err := config.ReadFromEnv(types.OsEnv{})
	if err != nil {
		panic(err)
	}
	log.Printf("faas-containerd starting..\tVersion: %s\tCommit: %s\tService Timeout: %s\n", Version, GitCommit, config.WriteTimeout.String())

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	writeHostsErr := ioutil.WriteFile(path.Join(wd, "hosts"),
		[]byte(`127.0.0.1	localhost`), 0644)

	if writeHostsErr != nil {
		log.Fatalln(fmt.Errorf("cannot write hosts file: %s", writeHostsErr).Error())
	}

	writeResolvErr := ioutil.WriteFile(path.Join(wd, "resolv.conf"),
		[]byte(`nameserver 8.8.8.8`), 0644)

	if writeResolvErr != nil {
		log.Fatalln(fmt.Errorf("cannot write resolv.conf file: %s", writeResolvErr).Error())
	}

	serviceMap := handlers.NewServiceMap()

	client, err := containerd.New(providerConfig.Sock)
	if err != nil {
		panic(err)
	}

	defer client.Close()

	invokeResolver := handlers.NewInvokeResolver(serviceMap)

	bootstrapHandlers := types.FaaSHandlers{
		FunctionProxy:        proxy.NewHandlerFunc(*config, invokeResolver),
		DeleteHandler:        handlers.MakeDeleteHandler(client, serviceMap),
		DeployHandler:        handlers.MakeDeployHandler(client, serviceMap),
		FunctionReader:       handlers.MakeReadHandler(client, serviceMap),
		ReplicaReader:        handlers.MakeReplicaReaderHandler(client, serviceMap),
		ReplicaUpdater:       handlers.MakeReplicaUpdateHandler(client, serviceMap),
		UpdateHandler:        handlers.MakeUpdateHandler(client, serviceMap),
		HealthHandler:        func(w http.ResponseWriter, r *http.Request) {},
		InfoHandler:          handlers.MakeInfoHandler(Version, GitCommit),
		ListNamespaceHandler: listNamespaces(),
	}

	log.Printf("TCP port: %d\n", *config.TCPPort)
	bootstrap.Serve(&bootstrapHandlers, config)
}

func listNamespaces() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []string{""}
		out, _ := json.Marshal(list)
		w.Write(out)
	}
}
