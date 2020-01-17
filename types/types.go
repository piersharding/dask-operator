package types

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/go/log"
	"github.com/go-logr/logr"
	analyticsv1 "github.com/piersharding/dask-operator/api/v1"
)

// Image Default Container Image
var Image string

// PullPolicy Default image pull policy
var PullPolicy string

// DaskContext is the set of parameters to configures this instance
type DaskContext struct {
	JupyterIngress     string
	SchedulerIngress   string
	MonitorIngress     string
	Daemon             bool
	Jupyter            bool
	DisablePolicies    bool
	Namespace          string
	Name               string
	ServiceType        string
	Port               int
	BokehPort          int
	Replicas           int32
	Cluster            string
	Script             string
	ScriptType         string
	ScriptContents     string
	Report             bool
	ReportStorageClass string
	MountedFile        bool
	Image              string
	Repository         string
	Tag                string
	PullSecrets        interface{}
	PullPolicy         string
	NodeSelector       interface{}
	Affinity           interface{}
	Tolerations        interface{}
	Resources          interface{}
	VolumeMounts       interface{}
	Volumes            interface{}
	Env                interface{}
	JupyterImage       string
	JupyterPassword    string
	Scheduler          interface{}
	Worker             interface{}
	Notebook           interface{}
}

// SetConfig setup the configuration
func SetConfig(dask analyticsv1.Dask) DaskContext {

	context := DaskContext{
		JupyterIngress:     dask.Spec.JupyterIngress,
		SchedulerIngress:   dask.Spec.SchedulerIngress,
		MonitorIngress:     dask.Spec.MonitorIngress,
		Daemon:             dask.Spec.Daemon,
		Jupyter:            dask.Spec.Jupyter,
		DisablePolicies:    dask.Spec.DisablePolicies,
		Namespace:          dask.Namespace,
		Name:               dask.Name,
		ServiceType:        "ClusterIP",
		Port:               8786,
		BokehPort:          8787,
		Replicas:           dask.Spec.Replicas,
		Image:              dask.Spec.Image,
		Script:             "/notebook.ipynb",
		ScriptType:         "",
		ScriptContents:     "",
		Report:             false,
		ReportStorageClass: "standard",
		MountedFile:        false,
		PullSecrets:        dask.Spec.PullSecrets,
		PullPolicy:         dask.Spec.ImagePullPolicy,
		NodeSelector:       dask.Spec.NodeSelector,
		Affinity:           dask.Spec.Affinity,
		Tolerations:        dask.Spec.Tolerations,
		Resources:          dask.Spec.Resources,
		VolumeMounts:       dask.Spec.VolumeMounts,
		Volumes:            dask.Spec.Volumes,
		Env:                dask.Spec.Env,
		JupyterImage:       "jupyter/scipy-notebook:latest",
		JupyterPassword:    dask.Spec.JupyterPassword,
		Scheduler:          dask.Spec.Scheduler,
		Worker:             dask.Spec.Worker,
		Notebook:           dask.Spec.Notebook}

	// if dask.Spec.Daemon != nil {
	// 	context.Daemon = *dask.Spec.Daemon
	// }

	// if dask.Spec.Jupyter != nil {
	// 	context.Jupyter = *dask.Spec.Jupyter
	// }

	// default of 5 replicas for workers
	if dask.Spec.Replicas == 0 {
		context.Replicas = 5

	}
	if context.MonitorIngress == "" {
		context.MonitorIngress = "monitor.dask.local"
	}

	if context.JupyterPassword == "" {
		context.JupyterPassword = "password"
	}

	log.Debugf("context: %+v", context)
	return context
}

// ForNotebook - copy and arrange config values for Notebook
func (context *DaskContext) ForNotebook() DaskContext {
	out := new(DaskContext)
	out = context
	out.applySpecifics(context.Notebook.(*analyticsv1.DaskDeploymentSpec))
	return *out
}

// ForScheduler - copy and arrange config values for Scheduler
func (context *DaskContext) ForScheduler() DaskContext {
	out := new(DaskContext)
	out = context
	out.applySpecifics(context.Scheduler.(*analyticsv1.DaskDeploymentSpec))
	return *out
}

// ForWorker - copy and arrange config values for Worker
func (context *DaskContext) ForWorker() DaskContext {
	out := new(DaskContext)
	out = context
	out.applySpecifics(context.Worker.(*analyticsv1.DaskDeploymentSpec))
	// if reflect.TypeOf(context.Worker) == reflect.TypeOf(&analyticsv1.DaskDeploymentSpec{}) {
	// 	if context.Worker.(*analyticsv1.DaskDeploymentSpec) != nil {
	return *out
}

// applySpecifics - copy and arrange config values for deployment class
func (context *DaskContext) applySpecifics(specific *analyticsv1.DaskDeploymentSpec) {

	if specific != nil {
		byt, _ := json.Marshal(specific)
		json.Unmarshal(byt, context)
		if specific.Volumes == nil {
			context.Volumes = nil
		}
		if specific.VolumeMounts == nil {
			context.VolumeMounts = nil
		}
		if specific.Env == nil {
			context.Env = nil
		}
		if specific.PullSecrets == nil {
			context.PullSecrets = nil
		}
		if specific.NodeSelector == nil {
			context.NodeSelector = nil
		}
		if specific.Affinity == nil {
			context.Affinity = nil
		}
		if specific.Tolerations == nil {
			context.Tolerations = nil
		}
		if specific.Resources == nil {
			context.Resources = nil
		}
	}
}

// SetJobConfig - add in DaskJob specific config elements
func (context *DaskContext) SetJobConfig(daskjob *analyticsv1.DaskJob) {
	if daskjob != nil {
		if daskjob.Spec.Image != "" {
			context.Image = daskjob.Spec.Image
		}
		if daskjob.Spec.ImagePullPolicy != "" {
			context.PullPolicy = daskjob.Spec.ImagePullPolicy
		}
		if daskjob.Spec.ReportStorageClass != "" {
			context.ReportStorageClass = daskjob.Spec.ReportStorageClass
		}
		context.Name = daskjob.Name
		context.Cluster = daskjob.Spec.Cluster
		context.Script = daskjob.Spec.Script
		context.Report = daskjob.Spec.Report
	}
}

// CustomLogger - add Errorf, Infof, and Debugf
type CustomLogger struct {
	logr.Logger
}

// WithValues helper
func (log *CustomLogger) WithValues(keysAndValues ...interface{}) CustomLogger {
	// fmt.Fprintf(os.Stderr, "in CustomLogger: %+v #\n", keysAndValues)
	// fmt.Fprintf(os.Stderr, "in CustomLogger: %+v #\n", log)
	return CustomLogger{Logger: log.Logger.WithValues(keysAndValues...)}
}

// Errorf helper
func (log *CustomLogger) Errorf(err error, format string, a ...interface{}) {
	log.Logger.Error(err, fmt.Sprintf(format, a...))
}

// Infof helper
func (log CustomLogger) Infof(format string, a ...interface{}) {
	log.Logger.Info(fmt.Sprintf(format, a...))
}

// Debugf helper
func (log CustomLogger) Debugf(format string, a ...interface{}) {
	log.Logger.Info(fmt.Sprintf(format, a...))
}
