package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

type HTTPAdmissionHandler interface {
	Handle(request *admissionv1.AdmissionReview, response *admissionv1.AdmissionReview) error
}

type AdmissionHandler struct {
	logger        klog.Logger
	schemeDecoder runtime.Decoder
	handler       HTTPAdmissionHandler
}

func NewAdmissionHandler(handler HTTPAdmissionHandler, logger klog.Logger) *AdmissionHandler {
	runtimeScheme := runtime.NewScheme()
	if err := admissionv1.AddToScheme(runtimeScheme); err != nil {
		logger.Error(err, "error adding AdmissionReview to scheme")
		os.Exit(1)
	}

	return &AdmissionHandler{
		logger:        logger,
		schemeDecoder: serializer.NewCodecFactory(runtimeScheme).UniversalDeserializer(),
		handler:       handler,
	}
}

func (ah *AdmissionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	admissionReview, err := ah.parseAdmissionReview(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ah.logger.Info("Successfully decoded AdmissionReview")

	admissionReviewRequest := admissionReview.Request
	if admissionReviewRequest == nil {
		errMsg := "Expected admission review request but did not get one"
		ah.logger.Info(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	admissionReviewResponse := &admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{},
	}

	err = ah.handler.Handle(admissionReview, admissionReviewResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	admissionReviewResponse.Response.UID = admissionReviewRequest.UID

	respBytes, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		ah.logger.Error(err, "error marshaling response for admission review")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ah.logger.Info("admission review response json marshalled", "json", respBytes)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		ah.logger.Error(err, "error %s writing admission response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ah *AdmissionHandler) parseAdmissionReview(body io.ReadCloser) (*admissionv1.AdmissionReview, error) {
	requestBody, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	admissionReview := new(admissionv1.AdmissionReview)
	expectedAdmissionReviewGVK := schema.GroupVersionKind{
		Group:   "admission.k8s.io",
		Version: "v1",
		Kind:    "AdmissionReview",
	}

	_, err = decodeRequest(ah.schemeDecoder, requestBody, expectedAdmissionReviewGVK, admissionReview)
	if err != nil {
		return nil, err
	}

	return admissionReview, nil
}

func decodeRequest(decoder runtime.Decoder, request []byte, expectedGVK schema.GroupVersionKind, into runtime.Object) (schema.GroupVersionKind, error) {
	_, requestGVK, err := decoder.Decode(request, nil, into)
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

func handleError(err error, msg string, logger klog.Logger) error {
	logger.Error(err, msg)
	return errors.New(msg)
}
