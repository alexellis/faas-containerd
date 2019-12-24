package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/openfaas/faas-provider/proxy"

	"github.com/containerd/containerd"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/gorilla/mux"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	bootstrap "github.com/openfaas/faas-provider"
	"github.com/openfaas/faas-provider/types"
)

var serviceMap map[string]*net.IP
var functionUptime time.Duration

var (
	Version   string
	GitCommit string
)

func main() {
	Start()
}

// Start faas-containerd
func Start() {
	log.Printf("faas-containerd starting..\tVersion: %s\tCommit: %s\n", Version, GitCommit)

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
	config := types.FaaSConfig{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		ReadTimeout:         functionUptime,
		WriteTimeout:        functionUptime,
	}

	bootstrapHandlers := types.FaaSHandlers{
		FunctionProxy:        proxy.NewHandlerFunc(config, invokeResolver{}),
		DeleteHandler:        deleteHandler(),
		DeployHandler:        deployHandler(client),
		FunctionReader:       readHandler(),
		ReplicaReader:        replicaReader(),
		ReplicaUpdater:       func(w http.ResponseWriter, r *http.Request) {},
		UpdateHandler:        updateHandler(client),
		HealthHandler:        func(w http.ResponseWriter, r *http.Request) {},
		InfoHandler:          func(w http.ResponseWriter, r *http.Request) {},
		ListNamespaceHandler: listNamespaces(),
	}

	port := 8081

	timeout := time.Minute * 120

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

type invokeResolver struct {
}

func (invokeResolver) Resolve(functionName string) (url.URL, error) {
	fmt.Println("Resolve: ", functionName)
	serviceIP, ok := serviceMap[functionName]
	if !ok {
		return url.URL{}, fmt.Errorf("not found")
	}

	fmt.Println(functionName, "=", serviceIP)

	const watchdogPort = 8080

	urlStr := fmt.Sprintf("http://%s:%d", serviceIP, watchdogPort)

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, err
	}

	return *urlRes, nil
}

func listNamespaces() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []string{}
		out, _ := json.Marshal(list)
		w.Write(out)
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
				Replicas:          1,
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
		for k := range serviceMap {
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

func deployHandler(client *containerd.Client) func(w http.ResponseWriter, r *http.Request) {
	return updateHandler(client)
}

func updateHandler(client *containerd.Client) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		snapshotter := ""
		if val, ok := os.LookupEnv("snapshotter"); ok {
			snapshotter = val
		}

		w.WriteHeader(http.StatusOK)
		req := types.FunctionDeployment{}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(body))

		json.Unmarshal(body, &req)

		go func() {
			ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")
			req.Image = "docker.io/" + req.Image

			image, err := prepareImage(ctx, client, req.Image, snapshotter)
			if err != nil {
				log.Println(err)
				return
			}
			size, _ := image.Size(ctx)
			log.Printf("Deploy %s size: %d\n", image.Name(), size)

			envs := []string{}
			fprocessFound := false
			fprocess := "fprocess=" + req.EnvProcess
			if len(req.EnvProcess) > 0 {
				fprocessFound = true
			}

			for k, v := range req.EnvVars {
				if k == "fprocess" {
					fprocessFound = true
					fprocess = v
				} else {
					envs = append(envs, k+"="+v)
				}
			}
			if fprocessFound {
				envs = append(envs, fprocess)
			}
			fmt.Println("Envs", envs)

			hook := func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
				if s.Hooks == nil {
					s.Hooks = &specs.Hooks{}
				}

				netnsPath, err := exec.LookPath("netns")
				log.Printf("netnsPath: %s\n", netnsPath)
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

			snapshotter := ""
			if val, ok := os.LookupEnv("snapshotter"); ok {
				snapshotter = val
			}

			wd, _ := os.Getwd()

			mounts := []specs.Mount{}
			mounts = append(mounts, specs.Mount{
				Destination: "/etc/resolv.conf",
				Type:        "bind",
				Source:      path.Join(wd, "resolv.conf"),
				Options:     []string{"rbind", "ro"},
			})

			mounts = append(mounts, specs.Mount{
				Destination: "/etc/hosts",
				Type:        "bind",
				Source:      path.Join(wd, "hosts"),
				Options:     []string{"rbind", "ro"},
			})

			// CAP_NET_RAW enable ping
			container, err := client.NewContainer(
				ctx,
				id,
				containerd.WithImage(image),
				containerd.WithSnapshotter(snapshotter),
				containerd.WithNewSnapshot(req.Service+"-snapshot", image),
				containerd.WithNewSpec(oci.WithImageConfig(image),
					oci.WithCapabilities([]string{"CAP_NET_RAW"}),
					oci.WithMounts(mounts),
					oci.WithEnv(envs),
					hook),
			)

			if err != nil {
				log.Println("Error starting container", err)
				return
			}

			defer container.Delete(ctx, containerd.WithSnapshotCleanup)

			defer func() {
				delete(serviceMap, req.Service)
			}()

			// create a task from the container
			task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
			if err != nil {
				log.Println("Error starting task", err)
				return
			}

			log.Printf("Container ID: %s\tTask ID %s:\tTask PID: %d\t\n", container.ID(), task.ID(), task.Pid())

			// https://github.com/weaveworks/weave/blob/master/net/netdev.go
			processID := task.Pid()
			bridge := "netns0"
			peerIDs, err := ConnectedToBridgeVethPeerIds(bridge)
			if err != nil {
				log.Fatalf("Unable to find peers on: %s %s", bridge, err)
			}

			addrs, addrsErr := GetNetDevsByVethPeerIds(int(processID), peerIDs)
			if addrsErr != nil {
				log.Fatalf("Unable to find address for veth pair using: %v %s", peerIDs, addrsErr)
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

			if err := task.Start(ctx); err != nil {
				log.Println("Error starting task", err)
				return
			}

			log.Printf("Waiting for task to exit")
			exitStatus := <-exitStatusC
			exitErr := "n/a"
			if exitStatus.Error() != nil {
				exitErr = exitStatus.Error().Error()
			}
			fmt.Printf("%s exitStatus: %d, error: %s\n", req.Service, exitStatus.ExitCode(), exitErr)

		}()

	}
}

func prepareImage(ctx context.Context, client *containerd.Client, imageName, snapshotter string) (containerd.Image, error) {

	var empty containerd.Image
	image, err := client.GetImage(ctx, imageName)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return empty, err
		}

		img, err := client.Pull(ctx, imageName, containerd.WithPullUnpack)
		if err != nil {
			return empty, fmt.Errorf("cannot pull: %s", err)
		}
		image = img
	}

	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return empty, fmt.Errorf("cannot check if unpacked: %s", err)
	}

	if !unpacked {
		if err := image.Unpack(ctx, snapshotter); err != nil {
			return empty, fmt.Errorf("cannot unpack: %s", err)
		}
	}

	return image, nil
}
