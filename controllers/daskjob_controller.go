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
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	analyticsv1 "gitlab.com/piersharding/dask-operator/api/v1"
	"gitlab.com/piersharding/dask-operator/models"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	"gitlab.com/piersharding/dask-operator/utils"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	jobOwnerKey     = ".metadata.controller"
	daskjobApiGVStr = analyticsv1.GroupVersion.String()
)

// DaskJobReconciler reconciles a DaskJob object
type DaskJobReconciler struct {
	client.Client
	Log       logr.Logger
	CustomLog dtypes.CustomLogger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=daskjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=analytics.piersharding.com,resources=daskjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile main reconcile loop
func (r *DaskJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("daskjob", req.NamespacedName)
	clog := r.CustomLog.WithValues("daskjob", req.NamespacedName)
	_ = clog

	var daskjob analyticsv1.DaskJob
	if err := r.Get(ctx, req.NamespacedName, &daskjob); err != nil {
		log.Info("unable to fetch DaskJob(delete in progress?): " + err.Error())
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	daskjob.Status.Succeeded = 0
	daskjob.Status.Resources = ""
	daskjob.Status.State = "Building"

	var dask analyticsv1.Dask
	daskobjkey := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      daskjob.Spec.Cluster,
	}
	if err := r.Get(ctx, daskobjkey, &dask); err != nil {
		log.Info("unable to fetch Dask(create/delete in progress?): " + err.Error())
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		daskjob.Status.State = fmt.Sprintf("Pending creation of Dask cluster: %s", daskjob.Spec.Cluster)
		r.Status().Update(ctx, &daskjob)
		return ctrl.Result{}, client.IgnoreNotFound(errors.New("unable to fetch DaskJob(create/delete in progress?): " + err.Error()))
	}

	if dask.Status.State != "Running" {
		log.Info(fmt.Sprintf("Dask cluster not ready: %s - %s - %s", daskjob.Spec.Cluster, dask.Status.State, dask.Status.Resources))
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("Dask cluster not ready: %s - %s - %s", daskjob.Spec.Cluster, dask.Status.State, dask.Status.Resources))
	}

	var currentJob *batchv1.Job
	var currentConfig *corev1.ConfigMap
	var currentJobConfig *corev1.ConfigMap
	var currentJobPVC *corev1.PersistentVolumeClaim

	var childJobs batchv1.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	// daskjob-job-app1
	currentJob, _ = r.getJob(daskjob.Namespace, "daskjob-job-"+daskjob.Name, &daskjob)
	currentConfig, _ = r.getConfig(dask.Namespace, "dask-configs-"+dask.Name)
	currentJobConfig, _ = r.getConfig(daskjob.Namespace, "daskjob-configs-"+daskjob.Name)
	currentJobPVC, _ = r.getPVC(daskjob.Namespace, "daskjob-report-pvc-"+daskjob.Name)

	_ = currentJob
	_ = currentConfig
	_ = currentJobConfig

	isJobFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		if job != nil && job.Status.Conditions != nil {
			for _, c := range job.Status.Conditions {
				if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
					return true, c.Type
				}
			}
		}

		return false, ""
	}
	// Compute status based on latest observed state.
	finishedState, finishedType := isJobFinished(currentJob)
	if finishedType == "" {
		daskjob.Status.State = "Running"
	} else if finishedState {
		daskjob.Status.State = string(finishedType)
	}
	Infof(log, "Status: %s", finishedType)

	Debugf(log, "incoming context: %+v", daskjob)

	// setup configuration.
	dcontext := dtypes.SetConfig(dask)
	dcontext.SetJobConfig(&daskjob)

	// check Script - is it a notebook, script, file or URL
	scriptType, scriptContents, mountedFile, err := utils.CheckJobScript(dcontext.Script)
	if err != nil {
		Errorf(log, err, "DaskJob script is invalid: %s", err.Error())
		r.Recorder.Eventf(&daskjob, corev1.EventTypeWarning, "Failed", "DaskJob script is invalid: %q", daskjob.Name)
		return ctrl.Result{Requeue: false, RequeueAfter: 0}, fmt.Errorf("DaskJob script is invalid: %s", err.Error())
	}
	dcontext.ScriptType = scriptType
	dcontext.ScriptContents = scriptContents
	dcontext.MountedFile = mountedFile

	// Get resource details
	resources, err := r.resourceDetails(dcontext)
	if err != nil {
		daskjob.Status.State = resources
		return ctrl.Result{}, err
	}
	daskjob.Status.Resources = resources

	// Generate desired children.

	// create dependent ConfigMap
	Debugf(log, "###### Create ConfigMap #######")
	if currentJobConfig == nil {
		configMap, err := models.DaskJobConfigs(dcontext)
		if err != nil {
			Errorf(log, err, "DaskJobConfigs Error: %+v\n", err)
			daskjob.Status.State = fmt.Sprintf("DaskJobConfigs Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		configMap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&daskjob, analyticsv1.GroupVersion.WithKind("DaskJob"))}

		Debugf(log, "DaskJobConfigs: %+v", *configMap)
		// set the reference
		if err := ctrl.SetControllerReference(&daskjob, configMap, r.Scheme); err != nil {
			Errorf(log, err, "DaskJobConfigs Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, configMap); err != nil {
			log.Error(err, "unable to create ConfigMap for Dask", "configMap", configMap)
			return ctrl.Result{}, err
		}
	}

	// create Job resources
	Debugf(log, "###### Create Job #######")
	if currentJob == nil {
		Debugf(log, "###### Create Job Report PVC #######")
		if dcontext.Report && currentJobPVC == nil {

			daskjobJobReportPVC, err := models.DaskJobReportStorage(dcontext)
			if err != nil {
				Errorf(log, err, "DaskJobReportStorage Error: %+v\n", err)
				daskjob.Status.State = fmt.Sprintf("DaskJobReportStorage Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			daskjobJobReportPVC.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&daskjob, analyticsv1.GroupVersion.WithKind("DaskJob"))}
			Debugf(log, "DaskJobReportStorage: %+v", *daskjobJobReportPVC)
			// set the reference
			if err := ctrl.SetControllerReference(&daskjob, daskjobJobReportPVC, r.Scheme); err != nil {
				Errorf(log, err, "DaskJobReportStorage Error: %+v\n", err)
				return ctrl.Result{}, err
			}
			// ...and create it on the cluster
			if err := r.Create(ctx, daskjobJobReportPVC); err != nil {
				log.Error(err, "unable to create Report PVC for DaskJob", "PVC", daskjobJobReportPVC)
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(&daskjob, corev1.EventTypeNormal, "Created", "Created Job Report PVC %q", daskjobJobReportPVC.Name)
		}

		// create job wide ServiceAccount
		daskjobServiceAccount, err := models.JobServiceAccount(dcontext)
		if err != nil {
			Errorf(log, err, "JobServiceAccount Error: %+v\n", err)
			daskjob.Status.State = fmt.Sprintf("JobServiceAccount Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		daskjobServiceAccount.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&daskjob, analyticsv1.GroupVersion.WithKind("DaskJob"))}
		Debugf(log, "JobServiceAccount: %+v", *daskjobServiceAccount)
		// set the reference
		if err := ctrl.SetControllerReference(&daskjob, daskjobServiceAccount, r.Scheme); err != nil {
			Errorf(log, err, "JobServiceAccount Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, daskjobServiceAccount); err != nil {
			log.Error(err, "unable to create JobServiceAccount for DaskJob", "ServiceAccount", daskjobServiceAccount)
			return ctrl.Result{}, err
		}

		// create Job
		daskjobJob, err := models.DaskJob(dcontext)
		if err != nil {
			Errorf(log, err, "DaskJob Error: %+v\n", err)
			daskjob.Status.State = fmt.Sprintf("DaskJob Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		daskjobJob.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&daskjob, analyticsv1.GroupVersion.WithKind("DaskJob"))}
		Debugf(log, "DaskJob: %+v", *daskjobJob)
		// set the reference
		if err := ctrl.SetControllerReference(&daskjob, daskjobJob, r.Scheme); err != nil {
			Errorf(log, err, "DaskJob Error: %+v\n", err)
			return ctrl.Result{}, err
		}
		// ...and create it on the cluster
		if err := r.Create(ctx, daskjobJob); err != nil {
			log.Error(err, "unable to create Job for DaskJob", "Job", daskjobJob)
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(&daskjob, corev1.EventTypeNormal, "Created", "Created Job %q", daskjobJob.Name)
	}

	// set the status and go home
	if err := r.Status().Update(ctx, &daskjob); err != nil {
		Errorf(log, err, "unable to update DaskJob status: %s", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager bootstrap reconciler
func (r *DaskJobReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(&batchv1.Job{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the Job object, extract the owner...
		job := rawObj.(*batchv1.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a DaskJob ...
		if owner.APIVersion != daskjobApiGVStr || owner.Kind != "DaskJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&analyticsv1.DaskJob{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
