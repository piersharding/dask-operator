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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	analyticsv1 "github.com/piersharding/dask-operator/api/v1"
	"github.com/piersharding/dask-operator/controllers"
	dtypes "github.com/piersharding/dask-operator/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// Debugf helper
func Debugf(log logr.Logger, format string, a ...interface{}) {
	log.Info(fmt.Sprintf(format, a...))
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = extv1beta1.AddToScheme(scheme)
	_ = analyticsv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme

	// set logging level
	logLevelVar, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to debug
	logLevel := false // will default to INFO
	if !ok {
		logLevel = true // Debug
	} else {
		switch logLevelVar {
		case "DEBUG":
			logLevel = true
		default:
			logLevel = false
		}
	}

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = logLevel
	}))

	log := ctrl.Log.WithName("controller-main").WithName("setup")

	// initialise core parameters
	image, ok := os.LookupEnv("IMAGE")
	if !ok {
		dtypes.Image = "daskdev/dask:latest"
	} else {
		dtypes.Image = image
	}
	Debugf(log, "Default Image: %s", dtypes.Image)

	pullPolicy, ok := os.LookupEnv("PULL_POLICY")
	if !ok {
		dtypes.PullPolicy = "IfNotPresent"
	} else {
		dtypes.PullPolicy = pullPolicy
	}
	Debugf(log, "Default PullPolicy: %s", dtypes.PullPolicy)

	Debugf(log, "Controller initialised.")
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var enableWebhooks bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableWebhooks, "enable-webhooks", false,
		"Enable WebHooks for the admission controller.")
	flag.Parse()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.DaskReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("Dask"),
		CustomLog: dtypes.CustomLogger{Logger: ctrl.Log.WithName("controllers").WithName("Dask")},
		Scheme:    mgr.GetScheme(),
		Recorder:  mgr.GetEventRecorderFor("dask-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Dask")
		os.Exit(1)
	}

	if enableWebhooks || os.Getenv("ENABLE_WEBHOOKS") == "true" {
		if err = (&analyticsv1.Dask{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Dask")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
