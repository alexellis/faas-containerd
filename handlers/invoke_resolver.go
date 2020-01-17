package handlers

import (
	"fmt"
	"log"
	"net/url"

	"github.com/containerd/containerd"
)

type InvokeResolver struct {
	client *containerd.Client
}

func NewInvokeResolver(client *containerd.Client) *InvokeResolver {
	return &InvokeResolver{client: client}
}

func (i *InvokeResolver) Resolve(functionName string) (url.URL, error) {
	log.Printf("Resolve: %q\n", functionName)

	fun, err := GetFunction(i.client, functionName)
	if err != nil {
		return url.URL{}, fmt.Errorf("not found")
	}
	serviceIP := fun.IP

	const watchdogPort = 8080

	urlStr := fmt.Sprintf("http://%s:%d", serviceIP, watchdogPort)

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, err
	}

	return *urlRes, nil
}
