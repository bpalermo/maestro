package handlers

import (
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/bpalermo/maestro/pkg/apis/config"
	configv1 "github.com/bpalermo/maestro/pkg/apis/config/v1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

type AdmissionValidationHandler struct {
	logger  klog.Logger
	decoder runtime.Decoder
}

func NewAdmissionValidationHandler(logger klog.Logger) (*AdmissionValidationHandler, error) {
	runtimeScheme := runtime.NewScheme()
	if err := admissionv1.AddToScheme(runtimeScheme); err != nil {
		return nil, err
	}

	return &AdmissionValidationHandler{
		logger,
		serializer.NewCodecFactory(runtimeScheme).UniversalDeserializer(),
	}, nil
}

func (avh AdmissionValidationHandler) Handle(request *admissionv1.AdmissionReview, response *admissionv1.AdmissionReview) (err error) {
	switch request.Request.Resource {
	case configv1.ProxyConfigMetaGVR:
		err = avh.handleProxyConfigReviewRequest(request.Request, response)
	default:
		return fmt.Errorf("expected config.maestro.io/v1/proxyconfig resource but got %+v", request)
	}

	return nil
}

func (avh AdmissionValidationHandler) handleProxyConfigReviewRequest(request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionReview) error {
	proxyConfigRequest := new(configv1.ProxyConfig)
	expectedProxyConfigGVK := schema.GroupVersionKind{
		Group:   config.GroupName,
		Version: "v1",
		Kind:    "ProxyConfig",
	}
	if _, err := decodeRequest(avh.decoder, request.Object.Raw, expectedProxyConfigGVK, proxyConfigRequest); err != nil {
		return err
	}

	err := protovalidate.Validate(proxyConfigRequest.Spec)

	response.SetGroupVersionKind(expectedProxyConfigGVK)
	if err != nil {
		response.Response.Allowed = false
		response.Response.Result = &metav1.Status{
			Status:  "Failure",
			Message: err.Error(),
		}
	} else {
		response.Response.Allowed = true
	}

	return nil
}
