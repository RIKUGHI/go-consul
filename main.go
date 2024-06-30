package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

const (
	ttl     = time.Second * 8
	checkID = "check_health"
)

type Service struct {
	consuleClient *api.Client
	Port          int
}

func NewService() *Service {
	client, err := api.NewClient(&api.Config{})
	if err != nil {
		log.Fatal(err)
	}
	return &Service{
		consuleClient: client,
	}
}

func (s *Service) Start() {
	ln, err := net.Listen("tcp", ":0") // Use ":0" to let the system assign an available port
	if err != nil {
		log.Fatal(err)
	}

	addr := ln.Addr().(*net.TCPAddr)
	s.Port = addr.Port // Capture the assigned port

	s.registerService()
	go s.updateHealthCheck()
	s.acceptLoop(ln)
}

func (s *Service) acceptLoop(ln net.Listener) {
	for {
		_, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (s *Service) updateHealthCheck() {
	ticker := time.NewTicker(time.Second * 5)
	for {
		err := s.consuleClient.Agent().UpdateTTL(checkID, "online", api.HealthPassing)
		if err != nil {
			log.Fatal(err)
		}
		<-ticker.C
	}
}

func (s *Service) registerService() {
	register := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("login_service_%d", s.Port), // Unique ID based on the port,
		Name:    "mycluster",
		Tags:    []string{"login"},
		Address: "127.0.01",
		Port:    s.Port,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: ttl.String(),
			TLSSkipVerify:                  true,
			TTL:                            ttl.String(),
			CheckID:                        checkID,
		},
	}

	query := map[string]any{
		"type":        "service",
		"service":     "mycluster",
		"passingonly": true,
	}

	plan, err := watch.Parse(query)
	if err != nil {
		log.Fatal(err)
	}

	plan.HybridHandler = func(index watch.BlockingParamVal, result any) {
		switch msg := result.(type) {
		case []*api.ServiceEntry:
			for _, entry := range msg {
				fmt.Println("new member joined", entry.Service)
			}
		}
	}
	go func() {
		plan.RunWithConfig("", &api.Config{})
	}()

	if err := s.consuleClient.Agent().ServiceRegister(register); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Service registered with Consul")
}

func main() {
	s := NewService()
	s.Start()
}
