package types

import "fmt"

type Endpoint struct {
	IP       string
	Port     uint32
	Protocol string
}

func NewEndpoint(addr string, port *int32, protocol *string) *Endpoint {
	return &Endpoint{
		IP:       addr,
		Port:     uint32(*port),
		Protocol: *protocol,
	}
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("%s:%s:%d", e.Protocol, e.IP, e.Port)
}
