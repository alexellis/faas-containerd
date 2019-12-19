package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/containerd/containerd"
	"github.com/gorilla/mux"
	"github.com/openfaas/faas-provider/types"
)

func MakeReplicaReaderHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		functionName := vars["name"]

		if addr := serviceMap.Get(functionName); addr != nil {
			found := types.FunctionStatus{
				Name:              functionName,
				AvailableReplicas: 1,
				Replicas:          1,
				// Namespace:         "openfaas-fn",
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
