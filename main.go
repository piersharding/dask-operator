package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/piersharding/dask-operator/models"
	dtypes "github.com/piersharding/dask-operator/types"

	ejson "encoding/json"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Image Default Container Image
var Image string

// PullPolicy Default image pull policy
var PullPolicy string

// ClientSet Global client connection handle
var ClientSet *kubernetes.Clientset

// Initialise logging etc.
func init() {

	// set logging level
	logLevelVar, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	if !ok {
		logLevelVar = "debug"
	}
	// parse string, this is built-in feature of logrus
	logLevel, err := log.ParseLevel(logLevelVar)
	if err != nil {
		logLevel = log.DebugLevel
	}
	// set global log level
	log.SetLevel(logLevel)

	// initialise core parameters
	Image, ok = os.LookupEnv("IMAGE")
	if !ok {
		Image = "daskdev/dask:latest"
	}
	log.Debugf("Default Image: %s", Image)

	PullPolicy, ok = os.LookupEnv("PULL_POLICY")
	if !ok {
		PullPolicy = "IfNotPresent"
	}
	log.Debugf("Default PullPolicy: %s", PullPolicy)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Debugf("No cluster config: %s", err.Error())
		var kubeconfig *string
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatalf("No current context config: %s", err.Error())
		}
	}
	log.Debugf("Current context config: %+v", config)

	// creates the clientset
	ClientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("ClientSet failed: %s", err.Error())
	}
	log.Debugf("ClientSet established")

	log.Debugf("initialised.")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// read back the status info for the Ingress resource
func ingressStatus(context dtypes.DaskContext) (string, error) {

	if context.JupyterIngress == "" && context.SchedulerIngress == "" {
		return "", nil
	}
	ingress, err := ClientSet.ExtensionsV1beta1().Ingresses(context.Namespace).Get("dask-"+context.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("ingressStatus.Get Error: %+v\n", err.Error())
		if !strings.Contains(err.Error(), "not found") {
			return "", err
		}
	}
	log.Debugf("The ingress: %+v\n", ingress)
	// log.Debugf("The ingress status: %+v\n", ingress.Status.LoadBalancer.Ingress[0].IP)
	ip := ""
	for _, i := range ingress.Status.LoadBalancer.Ingress {
		ip = ip + i.IP
	}
	var h []string
	for _, r := range ingress.Spec.Rules {
		h = append(h, fmt.Sprintf("http://%s/", r.Host))
	}
	hosts := fmt.Sprintf("Ingress: %s IP: %s, Hosts: %s", ingress.Name, ip, strings.Join(h[:], ", "))

	status, err := json.Marshal(&ingress.Status)
	// status, err := json.Marshal(&ingress)
	if err != nil {
		log.Errorf("ingressStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	s := hosts + " status: " + string(status)
	log.Debugf("The ingress status: %s\n", s)
	return s, nil
}

// read back the status info for the Service resource
func serviceStatus(context dtypes.DaskContext, name string) (string, error) {

	service, err := ClientSet.CoreV1().Services(context.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("serviceStatus.Get Error: %+v\n", err.Error())
		if !strings.Contains(err.Error(), "not found") {
			return "", err
		}
	}
	log.Debugf("The service: %+v\n", service)
	var ports []string
	for _, p := range service.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%s/%d", p.Name, p.Port))
	}
	portList := fmt.Sprintf("Service: %s Type: %s, IP: %s, Ports: %s", service.Name, service.Spec.Type, service.Spec.ClusterIP, strings.Join(ports[:], ","))

	status, err := json.Marshal(&service.Status)
	// status, err := json.Marshal(&service)
	if err != nil {
		log.Errorf("serviceStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	s := portList + " status: " + string(status)
	log.Debugf("The service status: %s\n", s)
	return s, nil
}

// read back the status info for the Deployment resource
func deploymentStatus(context dtypes.DaskContext, name string) (string, error) {

	// pods, err := ClientSet.CoreV1().Pods("").List(metav1.ListOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	deployment, err := ClientSet.AppsV1().Deployments(context.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("deploymentStatus.Get Error: %+v\n", err.Error())
		if !strings.Contains(err.Error(), "not found") {
			return "", err
		}
	}
	log.Debugf("The deployment: %+v\n", deployment)
	status, err := json.Marshal(&deployment.Status)
	if err != nil {
		log.Errorf("deploymentStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	log.Debugf("The deployment status: %s\n", string(status))
	return string(status), nil
}

// pull together the resource details
func resourceDetails(context dtypes.DaskContext) (string, error) {
	resIngress, err := ingressStatus(context)
	if err != nil {
		log.Errorf("ingressStatus Error: %+v\n", err)
		return fmt.Sprintf("ingressStatus Error: %+v\n", err), err
	}
	// res, err = deploymentStatus(context, "dask-scheduler-"+context.Name)
	// res, err = deploymentStatus(context, "dask-worker-"+context.Name)
	// res, err = deploymentStatus(context, "jupyter-notebook-"+context.Name)
	resSchedulerService, err := serviceStatus(context, "dask-scheduler-"+context.Name)
	if err != nil {
		log.Errorf("serviceStatus scheduler Error: %+v\n", err)
		return fmt.Sprintf("serviceStatus scheduler Error: %+v\n", err), err
	}
	var resJupyterService = ""
	if context.Jupyter {
		resJupyterService, err = serviceStatus(context, "jupyter-notebook-"+context.Name)
		if err != nil {
			log.Errorf("serviceStatus notebook Error: %+v\n", err)
			return fmt.Sprintf("serviceStatus notebook Error: %+v\n", err), err
		}
	}
	return fmt.Sprintf("%s - %s - %s", resIngress, resSchedulerService, resJupyterService), nil
}

// Main handler for controller sync requests
func sync(request *dtypes.SyncRequest) (*dtypes.SyncResponse, error) {

	response := &dtypes.SyncResponse{}
	response.Status.State = "Building"

	// Compute status based on latest observed state.
	for _, deployment := range request.Children.Deployments {
		response.Status.Replicas++
		if deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			response.Status.Succeeded++
		}
	}
	if response.Status.Replicas == response.Status.Succeeded {
		response.Status.State = "Running"
	}
	log.Infof("Status replicas: %i, succeeded: %i", response.Status.Replicas, response.Status.Succeeded)

	// setup configuration.
	context := dtypes.SetConfig(request)
	if context.Image == "" {
		context.Image = Image
	}
	if context.PullPolicy == "" {
		context.PullPolicy = PullPolicy
	}

	// Get resource details
	resources, err := resourceDetails(context)
	if err != nil {
		response.Status.State = resources
		return response, err
	}
	response.Status.Resources = resources

	// Generate desired children.
	configMap, err := models.DaskConfigs(context)
	if err != nil {
		log.Errorf("DaskConfigs Error: %+v\n", err)
		response.Status.State = fmt.Sprintf("DaskConfigs Error: %+v\n", err)
		return response, err
	}
	log.Debugf("DaskConfigs: %+v", *configMap)
	response.Children = append(response.Children, configMap)

	if context.Jupyter {
		jupyterService, err := models.JupyterService(context)
		if err != nil {
			log.Errorf("JupyterService Error: %+v\n", err)
			response.Status.State = fmt.Sprintf("JupyterService Error: %+v\n", err)
			return response, err
		}
		log.Debugf("JupyterService: %+v", *jupyterService)
		response.Children = append(response.Children, jupyterService)

		jupyterDeployment, err := models.JupyterDeployment(context)
		if err != nil {
			log.Errorf("JupyterDeployment Error: %+v\n", err)
			response.Status.State = fmt.Sprintf("JupyterDeployment Error: %+v\n", err)
			return response, err
		}
		log.Debugf("JupyterDeployment: %+v", *jupyterDeployment)
		response.Children = append(response.Children, jupyterDeployment)
	}

	schedulerService, err := models.DaskSchedulerService(context)
	if err != nil {
		log.Errorf("DaskSchedulerService Error: %+v\n", err)
		response.Status.State = fmt.Sprintf("DaskSchedulerService Error: %+v\n", err)
		return response, err
	}
	log.Debugf("DaskSchedulerService: %+v", *schedulerService)
	response.Children = append(response.Children, schedulerService)

	schedulerDeployment, err := models.DaskSchedulerDeployment(context)
	if err != nil {
		log.Errorf("DaskSchedulerDeployment Error: %+v\n", err)
		response.Status.State = fmt.Sprintf("DaskSchedulerDeployment Error: %+v\n", err)
		return response, err
	}
	log.Debugf("DaskSchedulerDeployment: %+v", *schedulerDeployment)
	response.Children = append(response.Children, schedulerDeployment)

	workerDeployment, err := models.DaskWorkerDeployment(context)
	if err != nil {
		log.Errorf("DaskWorkerDeployment Error: %+v\n", err)
		response.Status.State = fmt.Sprintf("DaskWorkerDeployment Error: %+v\n", err)
		return response, err
	}
	log.Debugf("DaskWorkerDeployment: %+v", *workerDeployment)
	response.Children = append(response.Children, workerDeployment)

	if context.JupyterIngress != "" || context.SchedulerIngress != "" {
		daskIngress, err := models.DaskIngress(context)
		if err != nil {
			log.Errorf("DaskIngress Error: %+v\n", err)
			response.Status.State = fmt.Sprintf("DaskIngress Error: %+v\n", err)
			return response, err
		}
		log.Debugf("DaskIngress: %+v", *daskIngress)
		response.Children = append(response.Children, daskIngress)
	}

	return response, nil
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Debugf("Body: %s", body)

	request := &dtypes.SyncRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Infof("Sync request: %s/%s", request.Parent.Namespace, request.Parent.Name)
	prettyJSON, err := ejson.MarshalIndent(request, "", "    ")
	log.Debugf("Entire request(JSON): %s", prettyJSON)

	log.Debugf("Parent: %+v", request.Parent)

	// trap error of sync for later
	response, serr := sync(request)

	body, err = json.Marshal(&response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	prettyJSON, err = ejson.MarshalIndent(response, "", "    ")
	log.Debugf("Entire response(JSON): %s", prettyJSON)

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)

	if serr != nil {
		// http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Errorf("Sync request errored: %s/%s", request.Parent.Namespace, request.Parent.Name)
	} else {
		log.Infof("Sync request completed: %s/%s", request.Parent.Namespace, request.Parent.Name)
	}
}

func main() {
	http.HandleFunc("/sync", syncHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
