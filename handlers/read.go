package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/containerd/containerd"
	"github.com/openfaas/faas-provider/types"
)

func MakeReadHandler(client *containerd.Client, serviceMap *ServiceMap) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		res := []types.FunctionStatus{}
		for _, k := range serviceMap.Keys() {
			res = append(res, types.FunctionStatus{
				Name:     k,
				Replicas: 1,
			})
		}

		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)

	}
}
