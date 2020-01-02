package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/alexellis/faasd/pkg/service"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/openfaas/faas-provider/types"
)

func MakeUpdateHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		log.Printf("[Update] request: %s\n", string(body))

		req := types.FunctionDeployment{}
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Printf("[Update] error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}
		name := req.Service

		if addr := serviceMap.Get(name); addr == nil {
			msg := fmt.Sprintf("service %s not found", name)
			log.Printf("[Update] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")

		containerErr := service.Remove(ctx, client, name)
		if containerErr != nil {
			log.Printf("[Update] error removing %s, %s\n", name, containerErr)
			http.Error(w, containerErr.Error(), http.StatusInternalServerError)
			return
		}

		serviceMap.Delete(name)

		deployErr := deploy(ctx, req, client, serviceMap)
		if deployErr != nil {
			log.Printf("[Update] error deploying %s, error: %s\n", name, deployErr)
			http.Error(w, deployErr.Error(), http.StatusBadRequest)
			return
		}
	}

}
