package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GrumpyServerHandler listen to admission requests and serve responses
type GrumpyServerHandler struct {
}

func (gs *GrumpyServerHandler) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		fmt.Printf("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	fmt.Printf("Received request")

	if r.URL.Path != "/validate" {
		fmt.Printf("no validate")
		http.Error(w, "no validate", http.StatusBadRequest)
		return
	}

	arRequest := admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, &arRequest); err != nil {
		fmt.Printf("incorrect body")
		http.Error(w, "incorrect body", http.StatusBadRequest)
	}

	raw := arRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if err := json.Unmarshal(raw, &pod); err != nil {
		fmt.Printf("error deserializing pod")
		return
	}
	if pod.Name == "smooth-app" {
		return
	}

	arResponse := admissionv1.AdmissionReview{
		Response: &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "Keep calm and not add more crap in the cluster!",
			},
		},
	}
	resp, err := json.Marshal(arResponse)
	if err != nil {
		fmt.Printf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	fmt.Printf("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		fmt.Printf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}