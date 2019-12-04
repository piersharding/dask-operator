/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	analyticsv1 "github.com/piersharding/dask-operator/api/v1"
	"github.com/piersharding/dask-operator/models"

	dtypes "github.com/piersharding/dask-operator/types"
)

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = analyticsv1.GroupVersion.String()
)

// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=dasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=dasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions,resources=ingresses/status,verbs=get

// Errorf helper
func Errorf(log logr.Logger, err error, format string, a ...interface{}) {
	log.Error(err, fmt.Sprintf(format, a...))
}

// Infof helper
func Infof(log logr.Logger, format string, a ...interface{}) {
	log.Info(fmt.Sprintf(format, a...))
}

// Debugf helper
func Debugf(log logr.Logger, format string, a ...interface{}) {
	log.Info(fmt.Sprintf(format, a...))
}

// DaskReconciler reconciles a Dask object
type DaskReconciler struct {
	client.Client
	Log       logr.Logger
	CustomLog dtypes.CustomLogger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// read back the status info for the Ingress resource
func (r *DaskReconciler) ingressStatus(dcontext dtypes.DaskContext) (string, error) {
	ctx := context.Background()

	objkey := client.ObjectKey{
		Namespace: dcontext.Namespace,
		Name:      "dask-" + dcontext.Name,
	}

	log := r.Log.WithValues("ingress", objkey)
	if dcontext.JupyterIngress == "" && dcontext.SchedulerIngress == "" {
		return "", nil
	}
	ingress := extv1beta1.Ingress{}
	if err := r.Get(ctx, objkey, &ingress); err != nil {
		Errorf(log, err, "ingressStatus.Get Error: %+v\n", err.Error())
		return "", client.IgnoreNotFound(err)
	}
	owner := metav1.GetControllerOf(&ingress)
	if owner == nil {
		err := errors.New("ingressStatus.Get Error: owner empty")
		log.Error(err, "ingressStatus.Get Error: owner empty", owner)
		return "", err
	}
	// ...make sure it's a Dask...
	if owner.APIVersion != apiGVStr || owner.Kind != "Dask" {
		err := errors.New("ingressStatus.Get Error: wrong kind/owner")
		log.Error(err, fmt.Sprintf("ingressStatus.Get Error: [%s] wrong kind/owner: %s/%s", apiGVStr, owner.Kind, owner.APIVersion))
		return "", err
	}

	// ingress, err := ClientSet.ExtensionsV1beta1().Ingresses(dcontext.Namespace).Get("dask-"+dcontext.Name, metav1.GetOptions{})
	// if err != nil {
	// 	log.Errorf("ingressStatus.Get Error: %+v\n", err.Error())
	// 	if !strings.Contains(err.Error(), "not found") {
	// 		return "", err
	// 	}
	// }
	Debugf(log, "The ingress: %+v\n", ingress)
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
		Errorf(log, err, "ingressStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	s := hosts + " status: " + string(status)
	Debugf(log, "The ingress status: %s\n", s)
	return s, nil
}

// read back the status info for the Service resource
func (r *DaskReconciler) serviceStatus(dcontext dtypes.DaskContext, name string) (string, error) {
	ctx := context.Background()

	objkey := client.ObjectKey{
		Namespace: dcontext.Namespace,
		Name:      name,
	}

	log := r.Log.WithValues("service", objkey)

	service := corev1.Service{}
	if err := r.Get(ctx, objkey, &service); err != nil {
		Errorf(log, err, "serviceStatus.Get Error: %+v\n", err.Error())
		return "", client.IgnoreNotFound(err)
	}
	owner := metav1.GetControllerOf(&service)
	if owner == nil {
		err := errors.New("serviceStatus.Get Error: owner empty")
		log.Error(err, "serviceStatus.Get Error: owner empty", owner)
		return "", err
	}
	// ...make sure it's a Dask...
	if owner.APIVersion != apiGVStr || owner.Kind != "Dask" {
		err := errors.New("serviceStatus.Get Error: wrong kind/owner")
		log.Error(err, "serviceStatus.Get Error: wrong kind/owner", owner)
		return "", err
	}

	// service, err := ClientSet.CoreV1().Services(context.Namespace).Get(name, metav1.GetOptions{})
	// if err != nil {
	// 	log.Errorf("serviceStatus.Get Error: %+v\n", err.Error())
	// 	if !strings.Contains(err.Error(), "not found") {
	// 		return "", err
	// 	}
	// }
	// Debugf(log, "The service: %+v\n", service)
	var ports []string
	for _, p := range service.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%s/%d", p.Name, p.Port))
	}
	portList := fmt.Sprintf("Service: %s Type: %s, IP: %s, Ports: %s", service.Name, service.Spec.Type, service.Spec.ClusterIP, strings.Join(ports[:], ","))

	status, err := json.Marshal(&service.Status)
	// status, err := json.Marshal(&service)
	if err != nil {
		Errorf(log, err, "serviceStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	s := portList + " status: " + string(status)
	Debugf(log, "The service status: %s\n", s)
	return s, nil
}

// read back the status info for the Deployment resource
func (r *DaskReconciler) deploymentStatus(dcontext dtypes.DaskContext, name string) (string, error) {

	// pods, err := ClientSet.CoreV1().Pods("").List(metav1.ListOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
	ctx := context.Background()

	objkey := client.ObjectKey{
		Namespace: dcontext.Namespace,
		Name:      name,
	}

	log := r.Log.WithValues("deployment", objkey)

	deployment := appsv1.Deployment{}
	if err := r.Get(ctx, objkey, &deployment); err != nil {
		Errorf(log, err, "deploymentStatus.Get Error: %+v\n", err.Error())
		return "", client.IgnoreNotFound(err)
	}
	owner := metav1.GetControllerOf(&deployment)
	if owner == nil {
		err := errors.New("deploymentStatus.Get Error: owner empty")
		log.Error(err, "deploymentStatus.Get Error: owner empty", owner)
		return "", err
	}
	// ...make sure it's a Dask...
	if owner.APIVersion != apiGVStr || owner.Kind != "Dask" {
		err := errors.New("deploymentStatus.Get Error: wrong kind/owner")
		log.Error(err, "deploymentStatus.Get Error: wrong kind/owner", owner)
		return "", err
	}

	// deployment, err := ClientSet.AppsV1().Deployments(context.Namespace).Get(name, metav1.GetOptions{})
	// if err != nil {
	// 	log.Errorf("deploymentStatus.Get Error: %+v\n", err.Error())
	// 	if !strings.Contains(err.Error(), "not found") {
	// 		return "", err
	// 	}
	// }
	Debugf(log, "The deployment: %+v\n", deployment)
	status, err := json.Marshal(&deployment.Status)
	if err != nil {
		Errorf(log, err, "deploymentStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	Debugf(log, "The deployment status: %s\n", string(status))
	return string(status), nil
}

// pull together the resource details
func (r *DaskReconciler) resourceDetails(dcontext dtypes.DaskContext) (string, error) {

	log := r.Log.WithName("resourceDetails")

	resIngress, err := r.ingressStatus(dcontext)
	if err != nil {
		Errorf(log, err, "ingressStatus Error: %s", err.Error())
		return fmt.Sprintf("ingressStatus Error: %+v\n", err), err
	}
	// res, err = r.deploymentStatus(dcontext, "dask-scheduler-"+dcontext.Name)
	// res, err = r.deploymentStatus(dcontext, "dask-worker-"+dcontext.Name)
	// res, err = r.deploymentStatus(dcontext, "jupyter-notebook-"+dcontext.Name)
	resSchedulerService, err := r.serviceStatus(dcontext, "dask-scheduler-"+dcontext.Name)
	if err != nil {
		Errorf(log, err, "serviceStatus scheduler Error: %+v\n", err)
		return fmt.Sprintf("serviceStatus scheduler Error: %+v\n", err), err
	}
	var resJupyterService = ""
	if dcontext.Jupyter {
		resJupyterService, err = r.serviceStatus(dcontext, "jupyter-notebook-"+dcontext.Name)
		if err != nil {
			Errorf(log, err, "serviceStatus notebook Error: %+v\n", err)
			return fmt.Sprintf("serviceStatus notebook Error: %+v\n", err), err
		}
	}
	return fmt.Sprintf("%s - %s - %s", resIngress, resSchedulerService, resJupyterService), nil
}

// look up one of the deployments
func (r *DaskReconciler) getDeployment(namespace string, name string) (*appsv1.Deployment, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for deployment", name)
	deploymentKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	deployment := appsv1.Deployment{}
	if err := r.Get(ctx, deploymentKey, &deployment); err != nil {
		Infof(log, "deployment.Get Error: %+v\n", err.Error())
		return nil, err
	}
	return &deployment, nil
}

// read back the status info for the Ingress resource
func (r *DaskReconciler) getIngress(namespace string, name string) (*extv1beta1.Ingress, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for ingress", name)
	objkey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	ingress := extv1beta1.Ingress{}
	if err := r.Get(ctx, objkey, &ingress); err != nil {
		Errorf(log, err, "ingressStatus.Get Error: %+v\n", err.Error())
		return nil, err
	}

	return &ingress, nil
}

// Reconcile main reconcile loop
func (r *DaskReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("dask", req.NamespacedName)
	clog := r.CustomLog.WithValues("dask", req.NamespacedName)
	_ = clog

	var dask analyticsv1.Dask
	if err := r.Get(ctx, req.NamespacedName, &dask); err != nil {
		log.Info("unable to fetch Dask(delete in progress?): " + err.Error())
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var currentSchedulerDeployment *appsv1.Deployment
	var currentWorkerDeployment *appsv1.Deployment
	var currentJupyterDeployment *appsv1.Deployment
	var currentIngress *extv1beta1.Ingress

	dask.Status.Replicas = 0
	dask.Status.Succeeded = 0
	dask.Status.Resources = ""
	dask.Status.State = "Building"

	var childDeployments appsv1.DeploymentList
	if err := r.List(ctx, &childDeployments, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Deployments")
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	currentSchedulerDeployment, _ = r.getDeployment(dask.Namespace, "dask-scheduler-"+dask.Name)
	currentWorkerDeployment, _ = r.getDeployment(dask.Namespace, "dask-worker-"+dask.Name)
	currentJupyterDeployment, _ = r.getDeployment(dask.Namespace, "jupyter-notebook-"+dask.Name)
	currentIngress, _ = r.getIngress(dask.Namespace, "dask-"+dask.Name)
	// deploymentKey := client.ObjectKey{
	// 	Namespace: dask.Namespace,
	// 	Name:      "dask-scheduler-" + dask.Name,
	// }
	// // log := r.Log.WithValues("looking for worker deployment", deploymentKey)
	// // currentSchedulerDeployment := appsv1.Deployment{}
	// if err = r.Get(ctx, deploymentKey, &currentSchedulerDeployment); err != nil {
	// 	Infof(log, "currentSchedulerDeployment.Get Error: %+v/%+v\n", err.Error(), currentSchedulerDeployment)
	// 	// return ctrl.Result{}, err
	// }
	// deploymentKey = client.ObjectKey{
	// 	Namespace: dask.Namespace,
	// 	Name:      "dask-worker-" + dask.Name,
	// }
	// // currentWorkerDeployment := appsv1.Deployment{}
	// if err = r.Get(ctx, deploymentKey, &currentWorkerDeployment); err != nil {
	// 	Infof(log, "currentWorkerDeployment.Get Error: %+v\n", err.Error())
	// }
	// deploymentKey = client.ObjectKey{
	// 	Namespace: dask.Namespace,
	// 	Name:      "jupyter-notebook-" + dask.Name,
	// }
	// // currentJupyterDeployment := appsv1.Deployment{}
	// if err = r.Get(ctx, deploymentKey, &currentJupyterDeployment); err != nil {
	// 	Infof(log, "currentJupyterDeployment.Get Error: %+v\n", err.Error())
	// }

	// for _, deployment := range childDeployments.Items {
	// 	dask.Status.Replicas++
	// 	if deployment.Status.ReadyReplicas == deployment.Status.Replicas {
	// 		dask.Status.Succeeded++
	// 	}
	// 	if strings.HasPrefix(deployment.Name, "dask-scheduler-") {
	// 		currentSchedulerDeployment = &deployment
	// 	} else if strings.HasPrefix(deployment.Name, "dask-worker-") {
	// 		currentWorkerDeployment = &deployment
	// 	} else if strings.HasPrefix(deployment.Name, "jupyter-notebook-") {
	// 		currentJupyterDeployment = &deployment
	// 	} else {
	// 		log.Error(errors.New("unable to parse Deployment type"), "deployment", &deployment)
	// 	}
	// }

	// _ = currentSchedulerDeployment
	// _ = currentWorkerDeployment
	// _ = currentJupyterDeployment

	// Compute status based on latest observed state.
	if dask.Status.Replicas == dask.Status.Succeeded {
		dask.Status.State = "Running"
	}
	Infof(log, "Status replicas: %d, succeeded: %d", dask.Status.Replicas, dask.Status.Succeeded)

	Debugf(log, "incoming context: %+v", dask)

	// setup configuration.
	dcontext := dtypes.SetConfig(dask)
	if dcontext.Image == "" {
		dcontext.Image = dtypes.Image
	}
	if dcontext.PullPolicy == "" {
		dcontext.PullPolicy = dtypes.PullPolicy
	}

	// Get resource details
	resources, err := r.resourceDetails(dcontext)
	if err != nil {
		dask.Status.State = resources
		return ctrl.Result{}, err
	}
	dask.Status.Resources = resources

	// Generate desired children.

	// create dependent ConfigMap
	Debugf(log, "###### Create ConfigMap #######")
	if currentSchedulerDeployment == nil {
		configMap, err := models.DaskConfigs(dcontext)
		if err != nil {
			Errorf(log, err, "DaskConfigs Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("DaskConfigs Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		configMap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}

		Debugf(log, "DaskConfigs: %+v", *configMap)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, configMap, r.Scheme); err != nil {
			Errorf(log, err, "DaskConfigs Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, configMap); err != nil {
			log.Error(err, "unable to create ConfigMap for Dask", "configMap", configMap)
			return ctrl.Result{}, err
		}
	}

	// if we have Jupyter enabled, create deployment
	Debugf(log, "###### Create Jupyter #######")
	if dcontext.Jupyter && currentJupyterDeployment == nil {
		jupyterService, err := models.JupyterService(dcontext)
		if err != nil {
			Errorf(log, err, "JupyterService Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("JupyterService Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		jupyterService.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "JupyterService: %+v", *jupyterService)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, jupyterService, r.Scheme); err != nil {
			Errorf(log, err, "DaskConfigs Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, jupyterService); err != nil {
			log.Error(err, "unable to create jupyterService for Dask", "Service", jupyterService)
			return ctrl.Result{}, err
		}

		jupyterDeployment, err := models.JupyterDeployment(dcontext.ForNotebook())
		if err != nil {
			Errorf(log, err, "JupyterDeployment Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("JupyterDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		jupyterDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "JupyterDeployment: %+v", *jupyterDeployment)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, jupyterDeployment, r.Scheme); err != nil {
			Errorf(log, err, "JupyterDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, jupyterDeployment); err != nil {
			log.Error(err, "unable to create JupyterDeployment for Dask", "Deployment", jupyterDeployment)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&dask, corev1.EventTypeNormal, "Created", "Created Jupyter deployment %q", jupyterDeployment.Name)
	}

	// create scheduler deployment and service
	Debugf(log, "###### Create Scheduler #######")
	if currentSchedulerDeployment == nil {
		schedulerService, err := models.DaskSchedulerService(dcontext)
		if err != nil {
			Errorf(log, err, "DaskSchedulerService Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("DaskSchedulerService Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		schedulerService.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "DaskSchedulerService: %+v", *schedulerService)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, schedulerService, r.Scheme); err != nil {
			Errorf(log, err, "DaskSchedulerService Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, schedulerService); err != nil {
			log.Error(err, "unable to create Service for Dask", "Service", schedulerService)
			return ctrl.Result{}, err
		}

		schedulerDeployment, err := models.DaskSchedulerDeployment(dcontext.ForScheduler())
		if err != nil {
			Errorf(log, err, "DaskSchedulerDeployment Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("DaskSchedulerDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		schedulerDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "DaskSchedulerDeployment: %+v", *schedulerDeployment)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, schedulerDeployment, r.Scheme); err != nil {
			Errorf(log, err, "DaskSchedulerDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, schedulerDeployment); err != nil {
			log.Error(err, "unable to create Deployment for Dask", "Deployment", schedulerDeployment)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&dask, corev1.EventTypeNormal, "Created", "Created Scheduler deployment %q", schedulerDeployment.Name)
	}

	// create worker cluster
	Debugf(log, "###### Create Worker #######")
	if currentWorkerDeployment == nil {
		workerDeployment, err := models.DaskWorkerDeployment(dcontext.ForWorker())
		if err != nil {
			Errorf(log, err, "DaskWorkerDeployment Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("DaskWorkerDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		workerDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "DaskWorkerDeployment: %+v", *workerDeployment)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, workerDeployment, r.Scheme); err != nil {
			Errorf(log, err, "DaskWorkerDeployment Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, workerDeployment); err != nil {
			log.Error(err, "unable to create Deployment for Dask", "Deployment", workerDeployment)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&dask, corev1.EventTypeNormal, "Created", "Created Worker deployment %q", workerDeployment.Name)
	} else {
		// deploymentKey := client.ObjectKey{
		// 	Namespace: dask.Namespace,
		// 	Name:      "dask-worker-" + dask.Name,
		// }

		// log := r.Log.WithValues("looking for worker deployment", deploymentKey)

		// currentWorkerDeployment := appsv1.Deployment{}
		// if err := r.Get(ctx, deploymentKey, &currentWorkerDeployment); err != nil {
		// 	Errorf(log, err, "currentWorkerDeployment.Get Error: %+v\n", err.Error())
		// 	return ctrl.Result{}, err
		// }
		// check the replicas
		if *currentWorkerDeployment.Spec.Replicas != dcontext.Replicas {
			*currentWorkerDeployment.Spec.Replicas = dcontext.Replicas
			if err := r.Update(ctx, currentWorkerDeployment); err != nil {
				log.Error(err, "unable to update Deployment for Dask", "Deployment", currentWorkerDeployment)
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(&dask, corev1.EventTypeNormal, "Updated", "Updated Worker deployment %q", currentWorkerDeployment.Name)
		}
	}

	// if required, create Ingress for scheduler and jupyter
	Debugf(log, "###### Create Ingress #######")
	if currentIngress == nil {
		if dcontext.JupyterIngress != "" || dcontext.SchedulerIngress != "" {

			daskIngress, err := models.DaskIngress(dcontext)

			if err != nil {
				Errorf(log, err, "DaskIngress Error: %+v\n", err)
				dask.Status.State = fmt.Sprintf("DaskIngress Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			daskIngress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
			Debugf(log, "DaskIngress: %+v", *daskIngress)
			// set the reference
			if err := ctrl.SetControllerReference(&dask, daskIngress, r.Scheme); err != nil {
				Errorf(log, err, "DaskIngress Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, daskIngress); err != nil {
				log.Error(err, "unable to create DaskIngress for Dask", "Ingress", daskIngress)
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(&dask, corev1.EventTypeNormal, "Created", "Created Ingress deployment %q", daskIngress.Name)
		}
	}

	// set the status and go home
	if err := r.Status().Update(ctx, &dask); err != nil {
		Errorf(log, err, "unable to update Dask status: %s", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager bootstrap reconciler
func (r *DaskReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the Deployment object, extract the owner...
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Dask ...
		if owner.APIVersion != apiGVStr || owner.Kind != "Dask" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&analyticsv1.Dask{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&extv1beta1.Ingress{}).
		Complete(r)
}
