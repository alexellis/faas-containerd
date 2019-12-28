package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/alexellis/faas-containerd/handlers"
	"github.com/openfaas/faas-provider/proxy"

	"github.com/containerd/containerd"

	bootstrap "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/types"
)

var serviceTimeout time.Duration

var (
	Version   string
	GitCommit string
)

func main() {
	Start()
}

// Start faas-containerd
func Start() {
	serviceTimeout = time.Second * 60 * 1

	if val, ok := os.LookupEnv("service_timeout"); ok {
		timeVal, _ := time.ParseDuration(val)
		serviceTimeout = timeVal
	}

	log.Printf("faas-containerd starting..\tVersion: %s\tCommit: %s\tService Timeout: %s\n", Version, GitCommit, serviceTimeout.String())

	sock := os.Getenv("sock")
	if len(sock) == 0 {
		sock = "/run/containerd/containerd.sock"
	}

	wd, _ := os.Getwd()

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

	client, err := containerd.New(sock)
	if err != nil {
		panic(err)
	}

	defer client.Close()
	config := types.FaaSConfig{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		ReadTimeout:         serviceTimeout,
		WriteTimeout:        serviceTimeout,
	}

	invokeResolver := handlers.NewInvokeResolver(serviceMap)

	bootstrapHandlers := types.FaaSHandlers{
		FunctionProxy:        proxy.NewHandlerFunc(config, invokeResolver),
		DeleteHandler:        handlers.MakeDeleteHandler(client, serviceMap),
		DeployHandler:        handlers.MakeDeployHandler(client, serviceMap),
		FunctionReader:       handlers.MakeReadHandler(client, serviceMap),
		ReplicaReader:        handlers.MakeReplicaReaderHandler(client, serviceMap),
		ReplicaUpdater:       handlers.MakeReplicaUpdateHandler(client, serviceMap),
		UpdateHandler:        handlers.MakeUpdateHandler(client, serviceMap),
		HealthHandler:        func(w http.ResponseWriter, r *http.Request) {},
		InfoHandler:          func(w http.ResponseWriter, r *http.Request) {},
		ListNamespaceHandler: listNamespaces(),
	}

	port := 8081

	bootstrapConfig := types.FaaSConfig{
		ReadTimeout:     serviceTimeout,
		WriteTimeout:    serviceTimeout,
		TCPPort:         &port,
		EnableBasicAuth: false,
		EnableHealth:    true,
	}

	log.Printf("TCP port: %d\n", port)

	bootstrap.Serve(&bootstrapHandlers, &bootstrapConfig)
}

func listNamespaces() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []string{"openfaas-fn"}
		out, _ := json.Marshal(list)
		w.Write(out)
	}
}
