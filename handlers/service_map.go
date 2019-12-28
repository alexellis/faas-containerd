package handlers

import (
	"net"
	"sync"
)

type ServiceMap struct {
	services map[string]*net.IP
	lock     *sync.RWMutex
}

func NewServiceMap() *ServiceMap {

	return &ServiceMap{
		services: make(map[string]*net.IP),
		lock:     &sync.RWMutex{},
	}
}

func (s *ServiceMap) Add(name string, addr *net.IP) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.services[name] = addr
}

func (s *ServiceMap) Get(name string) *net.IP {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.services[name]
}

func (s *ServiceMap) Keys() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	keys := []string{}

	for k := range s.services {
		keys = append(keys, k)
	}
	return keys
}

func (s *ServiceMap) Delete(name string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.services, name)
}
