package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/bpalermo/maestro/internal/config"
	"github.com/bpalermo/maestro/internal/config/annotation"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

var (
	ignoredNamespaces = []string{
		metav1.NamespaceSystem,
		metav1.NamespacePublic,
	}
)

type AdmissionMutationHandler struct {
	logger  klog.Logger
	decoder runtime.Decoder
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewAdmissionMutationHandler(logger klog.Logger) (*AdmissionMutationHandler, error) {
	runtimeScheme := runtime.NewScheme()
	if err := admissionv1.AddToScheme(runtimeScheme); err != nil {
		return nil, err
	}

	return &AdmissionMutationHandler{
		logger,
		serializer.NewCodecFactory(runtimeScheme).UniversalDeserializer(),
	}, nil
}

func (h *AdmissionMutationHandler) Handle(request *admissionv1.AdmissionReview, response *admissionv1.AdmissionReview) error {
	podGVR := metav1.GroupVersionResource{
		Group:    "core",
		Version:  "v1",
		Resource: "pods",
	}

	if request.Request.Resource != podGVR {
		return errors.New("admission request is not of kind: Pod")
	}

	pod := corev1.Pod{}

	_, _, err := h.decoder.Decode(request.Request.Object.Raw, nil, &pod)
	if err != nil {
		return handleError(err, "unable to unmarshall request to deployment", h.logger)
	}

	response.SetGroupVersionKind(request.GroupVersionKind())

	return h.mutate(pod, response)
}

func (h *AdmissionMutationHandler) mutate(pod corev1.Pod, response *admissionv1.AdmissionReview) error {
	// determine whether to perform a mutation
	if !h.mutationRequired(ignoredNamespaces, &pod.ObjectMeta) {
		h.logger.Info("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		response.Response.Allowed = true
		return nil
	}

	podAnnotations := map[string]string{annotation.SidecarStatus: "injected"}

	sidecarConfig := config.NewSidecarConfig(pod.ObjectMeta.Annotations)

	patchBytes, err := createPatch(&pod, sidecarConfig, podAnnotations)
	if err != nil {
		return handleError(err, "unable to marshal patch into bytes", h.logger)
	}

	response.Response.Allowed = true

	patchType := admissionv1.PatchTypeJSONPatch
	response.Response.PatchType = &patchType
	response.Response.Patch = patchBytes

	return nil
}

// mutationRequired checks whether the target resource needs to be mutated
func (h *AdmissionMutationHandler) mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta) bool {
	// skip special kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			h.logger.Info("Skip mutation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	podAnnotations := metadata.GetAnnotations()

	status := podAnnotations[annotation.SidecarStatus]

	// determine whether to perform mutation based on annotation for the target resource
	var required bool
	if strings.ToLower(status) == "injected" {
		required = false
	} else {
		switch strings.ToLower(podAnnotations[annotation.SidecarInject]) {
		default:
			required = true
		case "n", "not", "false", "off":
			required = false
		}
	}

	h.logger.Info("Mutation policy for %v/%v: status: %q required:%v", metadata.Namespace, metadata.Name, status, required)
	return required
}

// create the mutation patch for resources
func createPatch(pod *corev1.Pod, sidecarConfig *config.SidecarConfig, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, addContainer(pod.Spec.Containers, sidecarConfig.Containers, "/spec/containers")...)
	patch = append(patch, addVolume(pod.Spec.Volumes, sidecarConfig.Volumes, "/spec/volumes")...)
	patch = append(patch, updateAnnotation(pod.Annotations, annotations)...)

	return json.Marshal(patch)
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

func addContainer(target, added []corev1.Container, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolume(target, added []corev1.Volume, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}
