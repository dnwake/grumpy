package webhook

import (
	"strings"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
        v1 "k8s.io/api/core/v1"	
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
	admit, message := processRequest(admissionRequest)
	writeAdmitResponse(w, http.StatusOK, admissionReview, admit, "")
}	

func processRequest (admissionRequest *admissionv1.AdmissionRequest) (bool, string) {
	pod, err := parsePod(admissionRequest.Object.Raw)
	if err != nil {
	     	return false, err.Error()
        }
	for _, c := range pod.Spec.Containers {
	    	if strings.HasSuffix(c.Image, ":bad") {
		    	return false, "You cannot use the tag 'bad' in a container."
		}
	}
 	return true, ""
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
//	fmt.Print("Incoming request content: ")
//	fmt.Print(string(body))
	if err != nil {
		return incomingReview, err
	}

	deserializer := codec.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &incomingReview); err != nil {
		return incomingReview, err
	}

	return incomingReview, nil
}

func parsePod(object []byte) (*v1.Pod, error) {
	var pod v1.Pod
	if err := json.Unmarshal(object, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}
