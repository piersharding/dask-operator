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
	"fmt"

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
	daskOwnerKey = ".metadata.controller"
	daskApiGVStr = analyticsv1.GroupVersion.String()
)

// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=dasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=dasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions,resources=ingresses/status,verbs=get

// DaskReconciler reconciles a Dask object
type DaskReconciler struct {
	client.Client
	Log       logr.Logger
	CustomLog dtypes.CustomLogger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
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
	var currentConfig *corev1.ConfigMap

	dask.Status.Replicas = 0
	dask.Status.Succeeded = 0
	dask.Status.Resources = ""
	dask.Status.State = "Building"

	var childDeployments appsv1.DeploymentList
	if err := r.List(ctx, &childDeployments, client.InNamespace(req.Namespace), client.MatchingFields{daskOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Deployments")
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	currentSchedulerDeployment, _ = r.getDeployment(dask.Namespace, "dask-scheduler-"+dask.Name, &dask)
	currentWorkerDeployment, _ = r.getDeployment(dask.Namespace, "dask-worker-"+dask.Name, &dask)
	currentJupyterDeployment, _ = r.getDeployment(dask.Namespace, "jupyter-notebook-"+dask.Name, &dask)
	currentIngress, _ = r.getIngress(dask.Namespace, "dask-"+dask.Name)
	currentConfig, _ = r.getConfig(dask.Namespace, "dask-configs-"+dask.Name)

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
	if currentConfig == nil {

		// create cluster wide DNS NetworkPolicy
		if !dcontext.DisablePolicies {
			dnsNetworkPolicy, err := models.DNSNetworkPolicy(dcontext)
			if err != nil {
				Errorf(log, err, "DNSNetworkPolicy Error: %+v\n", err)
				dask.Status.State = fmt.Sprintf("DNSNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			dnsNetworkPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
			Debugf(log, "DNSNetworkPolicy: %+v", *dnsNetworkPolicy)
			// set the reference
			if err := ctrl.SetControllerReference(&dask, dnsNetworkPolicy, r.Scheme); err != nil {
				Errorf(log, err, "DNSNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, dnsNetworkPolicy); err != nil {
				log.Error(err, "unable to create DNSNetworkPolicy for Dask", "NetworkPolicy", dnsNetworkPolicy)
				return ctrl.Result{}, err
			}
		}

		// create cluster wide ServiceAccount
		clusterServiceAccount, err := models.ClusterServiceAccount(dcontext)
		if err != nil {
			Errorf(log, err, "ClusterServiceAccount Error: %+v\n", err)
			dask.Status.State = fmt.Sprintf("ClusterServiceAccount Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		clusterServiceAccount.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
		Debugf(log, "ClusterServiceAccount: %+v", *clusterServiceAccount)
		// set the reference
		if err := ctrl.SetControllerReference(&dask, clusterServiceAccount, r.Scheme); err != nil {
			Errorf(log, err, "ClusterServiceAccount Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, clusterServiceAccount); err != nil {
			log.Error(err, "unable to create ClusterServiceAccount for Dask", "ServiceAccount", clusterServiceAccount)
			return ctrl.Result{}, err
		}

		// create ConfigMap
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

		// create NetworkPolicy
		if !dcontext.DisablePolicies {
			jupyterNetworkPolicy, err := models.JupyterNetworkPolicy(dcontext.ForNotebook())
			if err != nil {
				Errorf(log, err, "JupyterNetworkPolicy Error: %+v\n", err)
				dask.Status.State = fmt.Sprintf("JupyterNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			jupyterNetworkPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
			Debugf(log, "JupyterNetworkPolicy: %+v", *jupyterNetworkPolicy)
			// set the reference
			if err := ctrl.SetControllerReference(&dask, jupyterNetworkPolicy, r.Scheme); err != nil {
				Errorf(log, err, "JupyterNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, jupyterNetworkPolicy); err != nil {
				log.Error(err, "unable to create JupyterNetworkPolicy for Dask", "NetworkPolicy", jupyterNetworkPolicy)
				return ctrl.Result{}, err
			}
		}

		// create Service
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

		// create Deployment
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

		// create NetworkPolicy
		if !dcontext.DisablePolicies {
			schedulerNetworkPolicy, err := models.DaskSchedulerNetworkPolicy(dcontext.ForNotebook())
			if err != nil {
				Errorf(log, err, "DaskSchedulerNetworkPolicy Error: %+v\n", err)
				dask.Status.State = fmt.Sprintf("DaskSchedulerNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			schedulerNetworkPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
			Debugf(log, "DaskSchedulerNetworkPolicy: %+v", *schedulerNetworkPolicy)
			// set the reference
			if err := ctrl.SetControllerReference(&dask, schedulerNetworkPolicy, r.Scheme); err != nil {
				Errorf(log, err, "DaskSchedulerNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, schedulerNetworkPolicy); err != nil {
				log.Error(err, "unable to create DaskSchedulerNetworkPolicy for Dask", "NetworkPolicy", schedulerNetworkPolicy)
				return ctrl.Result{}, err
			}
		}

		// create Service
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

		// create Deployment
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

		// create NetworkPolicy
		if !dcontext.DisablePolicies {
			workerNetworkPolicy, err := models.DaskWorkerNetworkPolicy(dcontext.ForNotebook())
			if err != nil {
				Errorf(log, err, "DaskWorkerNetworkPolicy Error: %+v\n", err)
				dask.Status.State = fmt.Sprintf("DaskWorkerNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			workerNetworkPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&dask, analyticsv1.GroupVersion.WithKind("Dask"))}
			Debugf(log, "DaskWorkerNetworkPolicy: %+v", *workerNetworkPolicy)
			// set the reference
			if err := ctrl.SetControllerReference(&dask, workerNetworkPolicy, r.Scheme); err != nil {
				Errorf(log, err, "DaskWorkerNetworkPolicy Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, workerNetworkPolicy); err != nil {
				log.Error(err, "unable to create DaskWorkerNetworkPolicy for Dask", "NetworkPolicy", workerNetworkPolicy)
				return ctrl.Result{}, err
			}
		}

		// create Deployment
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

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, daskOwnerKey, func(rawObj runtime.Object) []string {
		// grab the Deployment object, extract the owner...
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Dask ...
		if owner.APIVersion != daskApiGVStr || owner.Kind != "Dask" {
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
