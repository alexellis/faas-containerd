package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/openfaas/faas-provider/types"
)

func MakeReplicaUpdateHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		fmt.Printf("[Scale] request: %s\n", string(body))

		req := types.ScaleServiceRequest{}
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Printf("[Scale] error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		name := req.ServiceName

		if addr := serviceMap.Get(name); addr == nil {
			msg := fmt.Sprintf("service %s not found", name)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")

		ctr, ctrErr := client.LoadContainer(ctx, name)
		if ctrErr != nil {
			msg := fmt.Sprintf("cannot load service %s, error: %s", name, ctrErr)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		task, taskErr := ctr.Task(ctx, nil)
		if taskErr != nil {
			msg := fmt.Sprintf("cannot load task for service %s, error: %s", name, taskErr)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}
		var scaleErr error
		if req.Replicas == 0 {
			scaleErr = task.Pause(ctx)
		} else if req.Replicas == 1 {
			scaleErr = task.Resume(ctx)
		}

		if scaleErr != nil {
			msg := fmt.Sprintf("cannot scale task for %s, error: %s", name, scaleErr)
			log.Printf("[Scale] %s\n", msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
	}

}
