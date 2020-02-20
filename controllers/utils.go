package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	analyticsv1 "gitlab.com/piersharding/dask-operator/api/v1"
	dtypes "gitlab.com/piersharding/dask-operator/types"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

// set ownership and reference
func setOwnerReferences(res metav1.Object, cntlr metav1.Object, parentKind string, s *runtime.Scheme, t metav1.TypeMeta, log logr.Logger) error {

	// set ownership
	res.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cntlr, analyticsv1.GroupVersion.WithKind(parentKind))})

	Debugf(log, "%s: %+v\n", t.GetObjectKind(), res)
	// set the reference
	if err := ctrl.SetControllerReference(cntlr, res, s); err != nil {
		Errorf(log, err, "%s Error: %+v\n", t.GetObjectKind(), err)
		return err
	}
	return nil
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
	if owner.APIVersion != daskApiGVStr || owner.Kind != "Dask" {
		err := errors.New("ingressStatus.Get Error: wrong kind/owner")
		log.Error(err, fmt.Sprintf("ingressStatus.Get Error: [%s] wrong kind/owner: %s/%s", daskApiGVStr, owner.Kind, owner.APIVersion))
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
	if owner.APIVersion != daskApiGVStr || owner.Kind != "Dask" {
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
	if owner.APIVersion != daskApiGVStr || owner.Kind != "Dask" {
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
	resS, err := r.deploymentStatus(dcontext, "dask-scheduler-"+dcontext.Name)
	resW, err := r.deploymentStatus(dcontext, "dask-worker-"+dcontext.Name)
	resN, err := r.deploymentStatus(dcontext, "jupyter-notebook-"+dcontext.Name)
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
	return fmt.Sprintf("%s - %s - %s\nScheduler: %s\nWorker: %s\nNotebook: %s", resIngress, resSchedulerService, resJupyterService, resS, resW, resN), nil
}

// look up one of the deployments
func (r *DaskReconciler) getDeployment(namespace string, name string, dask *analyticsv1.Dask) (*appsv1.Deployment, error) {
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
	dask.Status.Replicas++
	if deployment.Status.ReadyReplicas == deployment.Status.Replicas {
		dask.Status.Succeeded++
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

// read back the status info for the Config resource
func (r *DaskReconciler) getConfig(namespace string, name string) (*corev1.ConfigMap, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for config", name)
	objkey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	config := corev1.ConfigMap{}
	if err := r.Get(ctx, objkey, &config); err != nil {
		Errorf(log, err, "configStatus.Get Error: %+v\n", err.Error())
		return nil, err
	}

	return &config, nil
}

// look up one of the jobs
func (r *DaskJobReconciler) getJob(namespace string, name string, daskjob *analyticsv1.DaskJob) (*batchv1.Job, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for job", name)
	jobKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	job := batchv1.Job{}
	if err := r.Get(ctx, jobKey, &job); err != nil {
		Infof(log, "job.Get Error: %+v\n", err.Error())
		return nil, err
	}
	// if job.Status.ReadyReplicas == job.Status.Replicas {
	// 	daskjob.Status.Succeeded++
	// }
	return &job, nil
}

// read back the status info for the Config resource
func (r *DaskJobReconciler) getConfig(namespace string, name string) (*corev1.ConfigMap, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for config", name)
	objkey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	config := corev1.ConfigMap{}
	if err := r.Get(ctx, objkey, &config); err != nil {
		Errorf(log, err, "configStatus.Get Error: %+v\n", err)
		return nil, err
	}

	return &config, nil
}

// read back the status info for the PVC resource
func (r *DaskJobReconciler) getPVC(namespace string, name string) (*corev1.PersistentVolumeClaim, error) {
	ctx := context.Background()
	log := r.Log.WithValues("looking for PVC", name)
	objkey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, objkey, &pvc); err != nil {
		Errorf(log, err, "pvcStatus.Get Error: %+v\n", err)
		return nil, err
	}

	return &pvc, nil
}

// read back the status info for the Deployment resource
func (r *DaskJobReconciler) jobStatus(dcontext dtypes.DaskContext, name string) (string, error) {

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

	log := r.Log.WithValues("job", objkey)

	job := batchv1.Job{}
	if err := r.Get(ctx, objkey, &job); err != nil {
		Errorf(log, err, "jobStatus.Get Error: %+v\n", err.Error())
		return "", client.IgnoreNotFound(err)
	}
	owner := metav1.GetControllerOf(&job)
	if owner == nil {
		err := errors.New("jobStatus.Get Error: owner empty")
		log.Error(err, "jobStatus.Get Error: owner empty", owner)
		return "", err
	}
	// ...make sure it's a Dask...
	if owner.APIVersion != daskjobApiGVStr || owner.Kind != "DaskJob" {
		err := errors.New("jobStatus.Get Error: wrong kind/owner")
		log.Error(err, "jobStatus.Get Error: wrong kind/owner", owner)
		return "", err
	}

	Debugf(log, "The job: %+v\n", job)
	status, err := json.Marshal(&job.Status)
	if err != nil {
		Errorf(log, err, "jobStatus.json Error: %+v\n", err.Error())
		return "", err
	}
	Debugf(log, "The job status: %s\n", string(status))
	return string(status), nil
}

// pull together the resource details
func (r *DaskJobReconciler) resourceDetails(dcontext dtypes.DaskContext) (string, error) {

	log := r.Log.WithName("resourceDetails")

	resJob, err := r.jobStatus(dcontext, "daskjob-job-"+dcontext.Name)
	if err != nil {
		Errorf(log, err, "jobStatus Error: %+v\n", err)
		return fmt.Sprintf("jobStatus Error: %+v\n", err), err
	}
	return fmt.Sprintf("Job: %s ", resJob), nil
}
