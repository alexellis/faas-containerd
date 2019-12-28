package handlers

import (
	"fmt"
	"net/url"
)

type InvokeResolver struct {
	serviceMap *ServiceMap
}

func NewInvokeResolver(serviceMap *ServiceMap) *InvokeResolver {
	return &InvokeResolver{
		serviceMap: serviceMap,
	}
}

func (i *InvokeResolver) Resolve(functionName string) (url.URL, error) {
	fmt.Printf("Resolve: %q\n", functionName)

	serviceIP := i.serviceMap.Get(functionName)
	if serviceIP == nil {
		return url.URL{}, fmt.Errorf("not found")
	}

	const watchdogPort = 8080

	urlStr := fmt.Sprintf("http://%s:%d", serviceIP, watchdogPort)

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, err
	}

	return *urlRes, nil
}
