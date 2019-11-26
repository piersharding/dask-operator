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
// +kubebuilder:docs-gen:collapse=Apache License

// Go imports
package v1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var dasklog = logf.Log.WithName("dask-resource")

func (r *Dask) SetupWebhookWithManager(mgr ctrl.Manager) error {
	dasklog.Info("Activating Webhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-analytics-piersharding-com-v1-dask,mutating=true,failurePolicy=fail,groups=analytics.piersharding.com,resources=dasks,verbs=create;update,versions=v1,name=mdask.piersharding.com

var _ webhook.Defaulter = &Dask{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Dask) Default() {
	dasklog.Info("default", "name", r.Name)

	if r.Spec.Image == "" {
		r.Spec.Image = "jupyter/scipy-notebook:latest"
	}

	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = "IfNotPresent"
	}

	if r.Spec.Daemon == nil {
		r.Spec.Daemon = new(bool)
	}

	if r.Spec.Jupyter == nil {
		r.Spec.Jupyter = new(bool)
	}

	if r.Spec.Replicas == nil {
		r.Spec.Replicas = new(int32)
		*r.Spec.Replicas = 1
	}
	if r.Spec.JupyterPassword == "" {
		r.Spec.JupyterPassword = "password"
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-analytics-piersharding-com-v1-dask,mutating=false,failurePolicy=fail,groups=analytics.piersharding.com,resources=dasks,versions=v1,name=vdask.piersharding.com

var _ webhook.Validator = &Dask{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Dask) ValidateCreate() error {
	dasklog.Info("validate create", "name", r.Name)

	return r.validateDask()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Dask) ValidateUpdate(old runtime.Object) error {
	dasklog.Info("validate update", "name", r.Name)

	return r.validateDask()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Dask) ValidateDelete() error {
	dasklog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Dask) validateDask() error {
	var allErrs field.ErrorList
	if err := r.validateDaskName(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateDaskSpec(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "analytics.piersharding.com", Kind: "Dask"},
		r.Name, allErrs)
}

func (r *Dask) validateDaskSpec() *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	// return validateScheduleFormat(
	// 	r.Spec.Schedule,
	// 	field.NewPath("spec").Child("schedule"))
	return nil
}

func validateScheduleFormat(schedule string, fldPath *field.Path) *field.Error {
	// if _, err := cron.ParseStandard(schedule); err != nil {
	// 	return field.Invalid(fldPath, schedule, err.Error())
	// }
	return nil
}

func (r *Dask) validateDaskName() *field.Error {
	if len(r.ObjectMeta.Name) > validationutils.DNS1035LabelMaxLength-11 {
		// The job name length is 63 character like all Kubernetes objects
		// (which must fit in a DNS subdomain). The cronjob controller appends
		// a 11-character suffix to the cronjob (`-$TIMESTAMP`) when creating
		// a job. The job name length limit is 63 characters. Therefore cronjob
		// names must have length <= 63-11=52. If we don't validate this here,
		// then job creation will fail later.
		return field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "must be no more than 52 characters")
	}
	return nil
}
