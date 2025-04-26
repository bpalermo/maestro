package v1

import (
	configv1 "github.com/bpalermo/maestro/api/maestro/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProxyConfig is a specification for a ProxyConfig resource
type ProxyConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *configv1.ProxyConfig `json:"spec"`
	Status ProxyConfigStatus     `json:"status"`
}

// ProxyConfigStatus is the status for a Foo resource
type ProxyConfigStatus struct {
	ResourceVersion string `json:"resourceVersion"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProxyConfigList is a list of ProxyConfig resources
type ProxyConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProxyConfig `json:"items"`
}
