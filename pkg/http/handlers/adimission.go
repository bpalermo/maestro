package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

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
	logger        klog.Logger
	schemeDecoder runtime.Decoder
}

func NewAdmissionValidationHandler(logger klog.Logger) AdmissionValidationHandler {
	runtimeScheme := runtime.NewScheme()
	if err := admissionv1.AddToScheme(runtimeScheme); err != nil {
		logger.Error(err, "error adding AdmissionReview to scheme")
		os.Exit(1)
	}

	codecFactory := serializer.NewCodecFactory(runtimeScheme)
	deserializer := codecFactory.UniversalDeserializer()

	return AdmissionValidationHandler{
		logger:        logger,
		schemeDecoder: deserializer,
	}
}

func (avh AdmissionValidationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}

	admissionReview := new(admissionv1.AdmissionReview)
	expectedAdmissionReviewGVK := schema.GroupVersionKind{
		Group:   "admission.k8s.io",
		Version: "v1",
		Kind:    "AdmissionReview",
	}

	admissionReviewGVK, err := avh.decodeRequest(requestBody, expectedAdmissionReviewGVK, admissionReview)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	avh.logger.Info("Successfully decoded AdmissionReview")

	admissionReviewRequest := admissionReview.Request
	if admissionReviewRequest == nil {
		errMsg := "Expected admission review request but did not get one"
		avh.logger.Info(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	admissionReviewResponse := &admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			UID: admissionReviewRequest.UID,
		},
	}

	switch admissionReviewRequest.Resource {
	case configv1.ProxyConfigMetaGVR:
		err = avh.handleProxyConfigReviewRequest(admissionReviewGVK, admissionReviewRequest, admissionReviewResponse)
	default:
		errMsg := fmt.Sprintf("Expected config.maestro.io/v1/proxyconfig resource but got %+v", admissionReviewRequest)
		slog.Error(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respBytes, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		avh.logger.Error(err, "error marshaling response for admission review")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	avh.logger.Info("admission review response json marshalled", "json", respBytes)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		avh.logger.Error(err, "error %s writing admission response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (avh AdmissionValidationHandler) decodeRequest(request []byte, expectedGVK schema.GroupVersionKind, into runtime.Object) (schema.GroupVersionKind, error) {
	_, requestGVK, err := avh.schemeDecoder.Decode(request, nil, into)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if requestGVK == nil {
		return schema.GroupVersionKind{}, errors.New("unable to find schema group, version and kind from request")
	}

	if *requestGVK != expectedGVK {
		errMsg := fmt.Sprintf(`Expected admission review with group: %s version: %s kind: %s 
but got group: %s version: %s kind: %s`, expectedGVK.Group, expectedGVK.Version, expectedGVK.Kind, requestGVK.Group, requestGVK.Version, requestGVK.Kind)
		return schema.GroupVersionKind{}, errors.New(errMsg)
	}
	return *requestGVK, nil
}

func (avh AdmissionValidationHandler) handleProxyConfigReviewRequest(gvk schema.GroupVersionKind, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionReview) error {
	proxyConfigRequest := new(configv1.ProxyConfig)
	expectedProxyConfigGVK := schema.GroupVersionKind{
		Group:   config.GroupName,
		Version: "v1",
		Kind:    "ProxyConfig",
	}
	if _, err := avh.decodeRequest(request.Object.Raw, expectedProxyConfigGVK, proxyConfigRequest); err != nil {
		return err
	}

	err := protovalidate.Validate(proxyConfigRequest.Spec)

	response.SetGroupVersionKind(gvk)
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
