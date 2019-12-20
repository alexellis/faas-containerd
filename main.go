package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/gorilla/mux"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	bootstrap "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/types"
)

var serviceMap map[string]*net.IP
var functionUptime time.Duration

func main() {

	log.Printf("faas-containerd starting..\n")

	sock := os.Getenv("sock")
	if len(sock) == 0 {
		sock = "/run/containerd/containerd.sock"
	}

	functionUptime = time.Second * 60 * 5

	if val, ok := os.LookupEnv("function_uptime"); ok {
		uptime, _ := time.ParseDuration(val)
		functionUptime = uptime
	}

	serviceMap = make(map[string]*net.IP)

	client, err := containerd.New(sock)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	bootstrapHandlers := types.FaaSHandlers{
		FunctionProxy:  invokeHandler(),
		DeleteHandler:  deleteHandler(),
		DeployHandler:  deployHandler(),
		FunctionReader: readHandler(),
		ReplicaReader:  replicaReader(),
		ReplicaUpdater: func(w http.ResponseWriter, r *http.Request) {},
		UpdateHandler:  updateHandler(client),
		HealthHandler:  func(w http.ResponseWriter, r *http.Request) {},
		InfoHandler:    func(w http.ResponseWriter, r *http.Request) {},
	}

	port := 8081

	timeout := time.Second * 60

	bootstrapConfig := types.FaaSConfig{
		ReadTimeout:     timeout,
		WriteTimeout:    timeout,
		TCPPort:         &port,
		EnableBasicAuth: false,
		EnableHealth:    true,
	}

	log.Printf("TCP port: %d\tTimeout: %s\tFunction uptime: %s\n",
		port,
		timeout.String(),
		functionUptime.String())

	bootstrap.Serve(&bootstrapHandlers, &bootstrapConfig)
}

func invokeHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		name := vars["name"]

		v, ok := serviceMap[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println(v, name)

		req, err := http.NewRequest(r.Method, "http://"+v.String()+":8080/", r.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()

		io.Copy(w, res.Body)
	}
}

func deleteHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK)

	}
}

func replicaReader() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		functionName := vars["name"]

		if _, ok := serviceMap[functionName]; ok {
			found := types.FunctionStatus{
				Name:              functionName,
				AvailableReplicas: 1,
			}

			functionBytes, _ := json.Marshal(found)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(functionBytes)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func readHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		res := []types.FunctionStatus{}
		for k, _ := range serviceMap {
			res = append(res, types.FunctionStatus{
				Name: k,
			})
		}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}

func deployHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK)

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(body))
	}
}

func updateHandler(client *containerd.Client) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK)
		req := types.FunctionDeployment{}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(body))

		json.Unmarshal(body, &req)

		go func() {
			ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")
			req.Image = "docker.io/" + req.Image

			image, err := client.Pull(ctx, req.Image, containerd.WithPullUnpack)
			if err != nil {
				log.Println(err)
				return
			}

			log.Println(image.Name())
			log.Println(image.Size(ctx))

			hook := func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
				if s.Hooks == nil {
					s.Hooks = &specs.Hooks{}
				}
				netnsPath, err := exec.LookPath("netns")
				if err != nil {
					return err
				}

				s.Hooks.Prestart = []specs.Hook{
					{
						Path: netnsPath,
						Args: []string{
							"netns",
						},
						Env: os.Environ(),
					},
				}
				return nil
			}

			id := req.Service

			// CAP_NET_RAW enable ping

			container, err := client.NewContainer(
				ctx,
				id,
				containerd.WithImage(image),
				containerd.WithNewSnapshot(req.Service+"-snapshot", image),
				containerd.WithNewSpec(oci.WithImageConfig(image), oci.WithCapabilities([]string{"CAP_NET_RAW"}), hook),
			)

			if err != nil {
				log.Println(err)
				return
			}

			defer container.Delete(ctx, containerd.WithSnapshotCleanup)

			defer func() {
				delete(serviceMap, req.Service)
			}()

			// create a task from the container
			task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
			if err != nil {
				log.Println(err)
				return
			}

			log.Printf("Container ID: %s\tTask ID %s:\tTask PID: %d\t\n", container.ID(), task.ID(), task.Pid())

			// https://github.com/weaveworks/weave/blob/master/net/netdev.go
			processID := task.Pid()
			peerIDs, err := ConnectedToBridgeVethPeerIds("netns0")
			if err != nil {
				log.Fatal(err)
			}

			addrs, addrsErr := GetNetDevsByVethPeerIds(int(processID), peerIDs)
			if addrsErr != nil {
				log.Fatal(addrsErr)
			}
			if len(addrs) > 0 {
				serviceMap[req.Service] = &addrs[0].CIDRs[0].IP
			}

			fmt.Println("Service IP: ", serviceMap[req.Service])

			defer task.Delete(ctx)

			// make sure we wait before calling start
			exitStatusC, err := task.Wait(ctx)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(exitStatusC)

			// call start on the task to execute the redis server
			if err := task.Start(ctx); err != nil {
				log.Println(err)
				return
			}

			// sleep for a bit to see the logs
			time.Sleep(functionUptime)

			// kill the process and get the exit status
			if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
				log.Println(err)
				return
			}

		}()

	}
}
