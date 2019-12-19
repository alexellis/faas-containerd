package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/alexellis/faasd/pkg/service"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openfaas/faas-provider/types"
	"github.com/pkg/errors"
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
	hook := prepareHook()
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
			oci.WithEnv(envs),
			hook),
	)

	if err != nil {
		return fmt.Errorf("unable to create container: %s, error: %s", name, err)
	}

	task, taskErr := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if taskErr != nil {
		return fmt.Errorf("unable to start task: %s, error: %s", name, taskErr)
	}

	log.Printf("Container ID: %s\tTask ID %s:\tTask PID: %d\t\n", container.ID(), task.ID(), task.Pid())

	ip, netErr := getIP("netns0", int(task.Pid()))

	if netErr != nil {
		return netErr
	}

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

func getIP(bridge string, pid int) (*net.IP, error) {
	// https://github.com/weaveworks/weave/blob/master/net/netdev.go

	peerIDs, err := ConnectedToBridgeVethPeerIds(bridge)
	if err != nil {
		return nil, fmt.Errorf("Unable to find peers on: %s %s", bridge, err)
	}

	addrs, addrsErr := GetNetDevsByVethPeerIds(pid, peerIDs)
	if addrsErr != nil {
		return nil, fmt.Errorf("Unable to find address for veth pair using: %v %s", peerIDs, addrsErr)
	}
	return &addrs[0].CIDRs[0].IP, nil

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

func prepareHook() func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
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
