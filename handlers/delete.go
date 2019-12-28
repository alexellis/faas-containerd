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

func MakeDeleteHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body == nil {
			http.Error(w, "expected a body", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)
		fmt.Printf("Deployment request:\n%s", string(body))

		req := types.FunctionDeployment{}
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Printf("[Delete] error parsing input: %s\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}
		name := req.Service

		if addr := serviceMap.Get(name); addr == nil {
			msg := fmt.Sprintf("service %s not found", name)
			log.Printf("[Delete] %s\n", msg)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		ctx := namespaces.WithNamespace(context.Background(), "openfaas-fn")

		containerErr := service.Remove(ctx, client, name)
		if containerErr != nil {
			log.Printf("[Delete] error removing %s, %s\n", name, containerErr)
			http.Error(w, containerErr.Error(), http.StatusInternalServerError)
			return
		}

		serviceMap.Delete(name)

	}

}
