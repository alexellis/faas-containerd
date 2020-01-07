package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// defaultCNIConf is a CNI configuration that enables network access to containers (docker-bridge style)
var defaultCNIConf = fmt.Sprintf(`{
	"cniVersion": "0.4.0",
	"name": "%s",
	"type": "bridge",
	"bridge": "%s",
	"isGateway": true,
	"isDefaultGateway": true,
	"promiscMode": true,
	"ipMasq": true,
	"ipam": {
		"type": "host-local",
		"ranges": [
			[{
				"subnet": "%s"
			}]
		]
	}
}
`, handlers.DefaultNetworkName, handlers.DefaultBridgeName, handlers.DefaultSubnet)

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

	netConfig := path.Join(handlers.CNIConfDir, handlers.DefaultCNIConfFilename)

	if exists, _ := pathExists(netConfig); !exists {
		log.Printf("Writing network config...\n")

		if !dirExists(handlers.CNIConfDir) {
			if err := os.MkdirAll(handlers.CNIConfDir, 0755); err != nil {
				log.Fatalln(fmt.Errorf("cannot create directory: %s", handlers.CNIConfDir).Error())
			}
		}

		if err := ioutil.WriteFile(netConfig, []byte(defaultCNIConf), 644); err != nil {
			log.Fatalln(fmt.Errorf("cannot write network config: %s", handlers.DefaultCNIConfFilename).Error())
		}
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

	log.Printf("Listening on TCP port: %d\n", *config.TCPPort)
	bootstrap.Serve(&bootstrapHandlers, config)
}

func listNamespaces() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []string{""}
		out, _ := json.Marshal(list)
		w.Write(out)
	}
}

func dirEmpty(dirname string) (b bool) {
	if !dirExists(dirname) {
		return
	}

	f, err := os.Open(dirname)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// If the first file is EOF, the directory is empty
	if _, err = f.Readdir(1); err == io.EOF {
		b = true
	}
	return
}

func dirExists(dirname string) bool {
	exists, info := pathExists(dirname)
	if !exists {
		return false
	}

	return info.IsDir()
}

func pathExists(path string) (bool, os.FileInfo) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	return true, info
}
