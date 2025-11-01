package types

import "fmt"

type ServiceID string

func NewServiceID(serviceName string, namespace string) ServiceID {
	return ServiceID(fmt.Sprintf("%s.%s", serviceName, namespace))
}

func (s ServiceID) ToString() string {
	return string(s)
}
