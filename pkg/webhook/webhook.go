package webhook

import (
	"fmt"
	"strings"	
	"encoding/json"
	"io/ioutil"
	"net/http"
        "os"
        "os/exec"	

	"git.dev.box.net/skynet/grumpy/pkg/patch"

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
	admit, message, patches := processRequest(admissionRequest)
	writeAdmitResponse(w, http.StatusOK, admissionReview, admit, message, patches)
}	

func processRequest (admissionRequest *admissionv1.AdmissionRequest) (bool, string, []patch.PatchOperation) {
	fmt.Printf("Processing Request\n")
	pod, err := parsePod(admissionRequest.Object.Raw)
	if err != nil {
	     	return false, err.Error(), nil
        }
	var patches []patch.PatchOperation
	for i, c := range pod.Spec.Containers {
		fmt.Printf("Processing Container Image '%s\n", c.Image)
		img, tag := parseImage(c.Image)
		cmd := fmt.Sprintf("/usr/local/notary-utils/notary-utils/bin/notary-lookup-without-env %s %s", img, tag)
		if output, err := runCmd(cmd); err != nil {
			annotationMessage := fmt.Sprintf("Unable to look up digest for image '%s'; error was '%s'\n", c.Image, err.Error())
			annotationPath := fmt.Sprintf("container.%d.image.error", i)
			patches = append(patches, patch.AddPatchOperation(fmt.Sprintf("/metadata/annotations/%s", annotationPath), annotationMessage))
			fmt.Printf(annotationMessage)
                } else {
		        newImage := fmt.Sprintf("%s@%s", img, strings.TrimSpace(output))
			path := fmt.Sprintf("/spec/containers/%d/image", i)
		    	patches = append(patches, patch.ReplacePatchOperation(path, newImage))
			annotationMessage := fmt.Sprintf("Image modified from '%s' to '%s' by mutation webhook\n", c.Image, newImage)
			annotation1Path := fmt.Sprintf("container.%d.image.mutated", i)
			annotation1Value := "true"
			annotation2Path := fmt.Sprintf("container.%d.image.original", i)
			annotation2Value := c.Image
			patches = append(patches, patch.AddPatchOperation(fmt.Sprintf("/metadata/annotations/%s", annotation1Path), annotation1Value))
			patches = append(patches, patch.AddPatchOperation(fmt.Sprintf("/metadata/annotations/%s", annotation2Path), annotation2Value))
			fmt.Printf("%s\n", annotationMessage)
		}
	}
 	return true, "", patches
}

func parseImage(image string) (string, string) {
	lastInd := strings.LastIndex(image, ":")
	if lastInd >= 0 {
            img := image[:lastInd]
	    tag := image[lastInd + 1:]
            return img, tag
        } else {
            img := image
            tag := "latest"
            return img, tag	    
        }
}


// writeAdmitResponse sends an allowed or disallowed response with additional message to the given admission request.
func writeAdmitResponse(w http.ResponseWriter, statusCode int, incomingReview admissionv1.AdmissionReview, isAllowed bool, message string, patches []patch.PatchOperation) {
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

	// set the patch operations for mutating admission
	if patches != nil && len(patches) > 0 {
		patchBytes, err := json.Marshal(patches)
		if err != nil {
			zap.L().Error("Error in marshaling outgoing admission review, returning 500", zap.Error(err))
         		w.WriteHeader(http.StatusInternalServerError)
			return
		}
		outgoingReview.Response.Patch = patchBytes

		// See https://stackoverflow.com/questions/35146286/find-address-of-constant-in-go
                patchType := admissionv1.PatchTypeJSONPatch
		outgoingReview.Response.PatchType = &patchType
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

func runCmd(command string) (string, error) {
    cmd := exec.Command("/bin/bash", "-c", command)
    cmd.Stdin = os.Stdin
    cmd.Stderr = os.Stderr
    stdOut, err := cmd.StdoutPipe()
    if err != nil {
        return "Could not set up stdout pipe", err
    }
    if err := cmd.Start(); err != nil {
        return "Could not start command", err
    }
    bytes, err := ioutil.ReadAll(stdOut)
    if err != nil {
        return "Could not read output", err
    }
    if err := cmd.Wait(); err != nil {
        return fmt.Sprintf("Could not wait for command; output was '%s'", string(bytes)), err
        if exitError, ok := err.(*exec.ExitError); ok {
	    if ec := exitError.ExitCode(); ec != 0 {
	        return fmt.Sprintf("Exit code was %d; output was '%s'", ec, string(bytes)), exitError
	    }
        }
    }
    return string(bytes), nil
}
