package config

import (
	"fmt"

	"github.com/bpalermo/maestro/internal/config/annotation"
	"github.com/bpalermo/maestro/internal/config/constants"
	corev1 "k8s.io/api/core/v1"
)

var (
	initContainerRestartPolicy = corev1.ContainerRestartPolicyAlways
)

type SidecarConfig struct {
	InitContainers []corev1.Container
	Containers     []corev1.Container
	Volumes        []corev1.Volume
}

func NewSidecarConfig(annotations map[string]string) *SidecarConfig {
	var initContainers, containers []corev1.Container
	var volumes []corev1.Volume

	serviceName := annotations[annotation.ServiceName]

	initContainers = append(initContainers, proxyContainer())

	volumes = append(volumes, proxyVolumes(serviceName)...)

	return &SidecarConfig{
		initContainers,
		containers,
		volumes,
	}
}

func proxyContainer() corev1.Container {
	return corev1.Container{
		Name:            "proxy",
		Image:           "envoyproxy/envoy:v1.32.4",
		ImagePullPolicy: corev1.PullAlways,
		RestartPolicy:   &initContainerRestartPolicy,
		Args: []string{
			"--service-cluster",
			"$(SERVICE_CLUSTER)",
			"--service-node",
			"$(SERVICE_NODE)",
			"--config-path",
			"/etc/envoy/envoy.yaml",
			"--log-level",
			"warn",
		},
		Env: []corev1.EnvVar{
			{
				Name: "SERVICE_CLUSTER",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.serviceAccountName",
					},
				},
			},
			{
				Name: "SERVICE_NODE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "admin",
				ContainerPort: 9901,
			},
			{
				Name:          "http",
				ContainerPort: 18080,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             &[]bool{true}[0],
			RunAsUser:                &[]int64{65532}[0],
			RunAsGroup:               &[]int64{65532}[0],
			AllowPrivilegeEscalation: &[]bool{false}[0],
			ReadOnlyRootFilesystem:   &[]bool{true}[0],
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "envoy-config",
				MountPath: "/var/envoy",
				ReadOnly:  true,
			},
			{
				Name:      "spiffe-workload-api",
				MountPath: "/spiffe-workload-api",
				ReadOnly:  true,
			},
		},
	}
}

func proxyVolumes(serviceName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "envoy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-envoy-bootstrap", serviceName),
					},
				},
			},
		},
		{
			Name: "spiffe-workload-api",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   constants.SpiffeCsiDriver,
					ReadOnly: &[]bool{true}[0],
				},
			},
		},
	}
}
