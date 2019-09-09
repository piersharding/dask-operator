package main

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/piersharding/dask-operator/models"

	ejson "encoding/json"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

type Controller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ControllerSpec   `json:"spec"`
	Status            ControllerStatus `json:"status"`
}

type ControllerSpec struct {
	Message    string `json:"message"`
	Replicas   int    `json:"replicas"`
	Daemon     bool   `json:"daemon"`
	Image      string `json:"image"`
	PullPolicy string `json:"imagePullPolicy"`
	PullSecret string `json:"imagePullSecrets"`
}

type ControllerStatus struct {
	Replicas  int `json:"replicas"`
	Succeeded int `json:"succeeded"`
}

type SyncRequest struct {
	Parent   Controller          `json:"parent"`
	Children SyncRequestChildren `json:"children"`
}

type SyncRequestChildren struct {
	Pods map[string]*v1.Pod `json:"Pod.v1"`
}

type SyncResponse struct {
	Status   ControllerStatus `json:"status"`
	Children []runtime.Object `json:"children"`
}

// Initialise logging etc.
func init() {

	// set logging level
	log_level_var, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		log_level_var = "debug"
	}
	// parse string, this is built-in feature of logrus
	log_evel, err := log.ParseLevel(log_level_var)
	if err != nil {
		log_evel = log.DebugLevel
	}
	// set global log level
	log.SetLevel(log_evel)

	log.Debugf("initialised.")

}

// Main handler for controller sync requests
func sync(request *SyncRequest) (*SyncResponse, error) {

	response := &SyncResponse{}

	// Compute status based on latest observed state.
	for _, pod := range request.Children.Pods {
		response.Status.Replicas += 1
		if pod.Status.Phase == v1.PodSucceeded {
			response.Status.Succeeded += 1
		}
	}

	// Generate desired children.
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: request.Parent.Name,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:    "hello",
					Image:   "busybox",
					Command: []string{"echo", request.Parent.Spec.Message},
				},
			},
		},
	}
	response.Children = append(response.Children, pod)

	return response, nil
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Debugf("Body: %s", body)

	request := &SyncRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	prettyJSON, err := ejson.MarshalIndent(request, "", "    ")
	log.Debugf("Children: %s", prettyJSON)

	log.Debugf("Parent: %+v", request.Parent)

	context := models.DaskContext{
		Daemon:       request.Parent.Spec.Daemon,
		Namespace:    request.Parent.Namespace,
		Name:         request.Parent.Name,
		ServiceType:  "ClusterIP",
		Port:         8786,
		BokehPort:    8787,
		Replicas:     request.Parent.Spec.Replicas,
		Image:        request.Parent.Spec.Image,
		PullSecret:   request.Parent.Spec.PullSecret,
		PullPolicy:   request.Parent.Spec.PullPolicy,
		NodeSelector: "",
		Affinity:     "",
		Tolerations:  ""}

	log.Debugf("context: %+v", context)

	resp, err := models.DaskSchedulerService(context)
	log.Debugf("DaskSchedulerService: %s", resp)

	resp, err = models.DaskSchedulerDeployment(context)
	log.Debugf("DaskSchedulerDeployment: %s", resp)

	response, err := sync(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err = json.Marshal(&response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func main() {
	http.HandleFunc("/sync", syncHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
