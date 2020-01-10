package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alexellis/faasd/pkg/service"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openfaas/faas-provider/types"
	"github.com/pkg/errors"
)

const (
	// TODO: CNIBinDir and CNIConfDir should maybe be globally configurable?
	// CNIBinDir describes the directory where the CNI binaries are stored
	CNIBinDir = "/opt/cni/bin"
	// CNIConfDir describes the directory where the CNI plugin's configuration is stored
	CNIConfDir = "/etc/cni/net.d"
	// netNSPathFmt gives the path to the a process network namespace, given the pid
	NetNSPathFmt = "/proc/%d/ns/net"

	// defaultCNIConfFilename is the vanity filename of default CNI configuration file
	DefaultCNIConfFilename = "10-openfaas.conflist"
	// defaultNetworkName names the "docker-bridge"-like CNI plugin-chain installed when no other CNI configuration is present.
	// This value appears in iptables comments created by CNI.
	DefaultNetworkName = "openfaas-cni-bridge"
	// defaultBridgeName is the default bridge device name used in the defaultCNIConf
	DefaultBridgeName = "openfaas0"
	// defaultSubnet is the default subnet used in the defaultCNIConf -- this value is set to not collide with common container networking subnets:
	DefaultSubnet = "10.62.0.0/16"
)

func MakeDeployHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		log.Printf("[Deploy] request: %s\n", string(body))

		req := types.FunctionDeployment{}
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Printf("[Deploy] - error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		name := req.Service

		ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")

		deployErr := deploy(ctx, req, client, serviceMap)
		if deployErr != nil {
			log.Printf("[Deploy] error deploying %s, error: %s\n", name, deployErr)
			http.Error(w, deployErr.Error(), http.StatusBadRequest)
			return
		}
	}
}

func deploy(ctx context.Context, req types.FunctionDeployment, client *containerd.Client, serviceMap *ServiceMap) error {

	imgRef := "docker.io/" + req.Image
	if strings.Index(req.Image, ":") == -1 {
		imgRef = imgRef + ":latest"
	}

	snapshotter := ""
	if val, ok := os.LookupEnv("snapshotter"); ok {
		snapshotter = val
	}

	image, err := service.PrepareImage(ctx, client, imgRef, snapshotter)
	if err != nil {
		return errors.Wrapf(err, "unable to pull image %s", imgRef)
	}

	size, _ := image.Size(ctx)
	log.Printf("Deploy %s size: %d\n", image.Name(), size)

	envs := prepareEnv(req.EnvProcess, req.EnvVars)
	mounts := getMounts()

	name := req.Service

	container, err := client.NewContainer(
		ctx,
		name,
		containerd.WithImage(image),
		containerd.WithSnapshotter(snapshotter),
		containerd.WithNewSnapshot(req.Service+"-snapshot", image),
		containerd.WithNewSpec(oci.WithImageConfig(image),
			oci.WithCapabilities([]string{"CAP_NET_RAW"}),
			oci.WithMounts(mounts),
			oci.WithEnv(envs)),
	)

	if err != nil {
		return fmt.Errorf("unable to create container: %s, error: %s", name, err)
	}

	task, taskErr := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if taskErr != nil {
		return fmt.Errorf("unable to start task: %s, error: %s", name, taskErr)
	}

	log.Printf("Container ID: %s\tTask ID %s:\tTask PID: %d\t\n", container.ID(), task.ID(), task.Pid())

	id := uuid.New().String()
	netns := fmt.Sprintf(NetNSPathFmt, task.Pid())

	cni, err := gocni.New(gocni.WithPluginConfDir(CNIConfDir),
		gocni.WithPluginDir([]string{CNIBinDir}))

	if err != nil {
		return err
	}

	// Load the cni configuration
	if err := cni.Load(gocni.WithLoNetwork, gocni.WithConfListFile(filepath.Join(CNIConfDir, DefaultCNIConfFilename))); err != nil {
		log.Fatalf("failed to load cni configuration: %v", err)
	}

	labels := map[string]string{}

	result, err := cni.Setup(ctx, id, netns, gocni.WithLabels(labels))
	if err != nil {
		return errors.Wrapf(err, "failed to setup network for namespace %q: %v", id, err)
	}

	// Get the IP of the default interface.
	defaultInterface := gocni.DefaultPrefix + "0"

	if _, ok := result.Interfaces[defaultInterface]; !ok {
		return fmt.Errorf("failed to find interface %q", defaultInterface)
	}
	if result.Interfaces[defaultInterface].IPConfigs != nil &&
		len(result.Interfaces[defaultInterface].IPConfigs) == 0 {
		return fmt.Errorf("failed to find IP for interface %q, no configs found", defaultInterface)
	}

	ip := &result.Interfaces[defaultInterface].IPConfigs[0].IP

	serviceMap.Add(name, ip)

	log.Printf("%s has IP: %s\n", name, ip.String())

	_, waitErr := task.Wait(ctx)
	if waitErr != nil {
		return errors.Wrapf(waitErr, "Unable to wait for task to start: %s", name)
	}

	if startErr := task.Start(ctx); startErr != nil {
		return errors.Wrapf(startErr, "Unable to start task: %s", name)
	}

	return nil
}

func prepareEnv(envProcess string, reqEnvVars map[string]string) []string {
	envs := []string{}
	fprocessFound := false
	fprocess := "fprocess=" + envProcess
	if len(envProcess) > 0 {
		fprocessFound = true
	}

	for k, v := range reqEnvVars {
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
	return envs
}

func getMounts() []specs.Mount {
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
	return mounts
}
