package webhook

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var codec = serializer.NewCodecFactory(runtime.NewScheme())

const (
	PodExecAdmissionRequestKind   = "PodExecOptions"
	PodAttachAdmissionRequestKind = "PodAttachOptions"
)

//GrumpyServerHandler listen to admission requests and serve responses
type GrumpyServerHandler struct {
}

func (gs *GrumpyServerHandler) Serve(w http.ResponseWriter, r *http.Request) {
	admissionReview, err := parseIncomingRequest(r)
	if err != nil || admissionReview.Request == nil {
		zap.L().Error("Received a bad request when admitting Pod interaction", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	admissionRequest := admissionReview.Request

	// parse the request into an PodInteraction object and add it to channel for controller to process
	podName := admissionRequest.Name

        admit := (podName == "smooth-app")

        //object :- admissionRequest.Object

	writeAdmitResponse(w, http.StatusOK, admissionReview, admit, "")
}

// writeAdmitResponse sends an allowed or disallowed response with additional message to the given admission request.
func writeAdmitResponse(w http.ResponseWriter, statusCode int, incomingReview admissionv1.AdmissionReview, isAllowed bool, message string) {
	w.Header().Set("Content-Type", "application/json")

	outgoingReview := admissionv1.AdmissionReview{
		TypeMeta: incomingReview.TypeMeta,
		Response: &admissionv1.AdmissionResponse{
			Allowed: isAllowed,
		},
	}

	if incomingReview.Request != nil {
		outgoingReview.Response.UID = incomingReview.Request.UID
	}

	// add a message with 403 HTTP status code when rejecting a request
	if !isAllowed {
		outgoingReview.Response.Result = &metav1.Status{
			Code:    http.StatusForbidden,
			Message: message,
		}
	}

	response, err := json.Marshal(outgoingReview)
	if err != nil {
		zap.L().Error("Error in marshaling outgoing admission review, returning 500", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(response); err != nil {
		zap.L().Error("Error in writing an admit response, returning 500", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
}

// parseIncomingRequest parses the incoming request body and returns an admission.AdmissionReview object.
func parseIncomingRequest(r *http.Request) (admissionv1.AdmissionReview, error) {
	defer r.Body.Close()

	var incomingReview admissionv1.AdmissionReview
	body, err := ioutil.ReadAll(r.Body)
	zap.L().Info("Incoming request content: ", zap.string("content", string(body)))
	if err != nil {
		return incomingReview, err
	}

	deserializer := codec.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &incomingReview); err != nil {
		return incomingReview, err
	}

	return incomingReview, nil
}

