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

var daskjoblog = logf.Log.WithName("daskjob-resource")

// SetupWebhookWithManager - bootstrap manager
func (r *DaskJob) SetupWebhookWithManager(mgr ctrl.Manager) error {
	daskjoblog.Info("Activating Webhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-analytics-piersharding-com-v1-daskjob,mutating=true,failurePolicy=fail,groups=analytics.piersharding.com,resources=daskjobs,verbs=create;update,versions=v1,name=mdaskjob.piersharding.com

var _ webhook.Defaulter = &DaskJob{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DaskJob) Default() {
	daskjoblog.Info("default", "name", r.Name)

	if r.Spec.Image == "" {
		r.Spec.Image = "jupyter/scipy-notebook:latest"
	}

	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = "IfNotPresent"
	}

}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-analytics-piersharding-com-v1-daskjob,mutating=false,failurePolicy=fail,groups=analytics.piersharding.com,resources=daskjobs,versions=v1,name=vdaskjob.piersharding.com

var _ webhook.Validator = &DaskJob{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DaskJob) ValidateCreate() error {
	daskjoblog.Info("validate create", "name", r.Name)

	return r.validateDaskJob()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DaskJob) ValidateUpdate(old runtime.Object) error {
	daskjoblog.Info("validate update", "name", r.Name)

	return r.validateDaskJob()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DaskJob) ValidateDelete() error {
	daskjoblog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *DaskJob) validateDaskJob() error {
	var allErrs field.ErrorList
	if err := r.validateDaskJobName(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateDaskJobSpec(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "analytics.piersharding.com", Kind: "DaskJob"},
		r.Name, allErrs)
}

func (r *DaskJob) validateDaskJobSpec() *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	// return validateScheduleFormat(
	// 	r.Spec.Schedule,
	// 	field.NewPath("spec").Child("schedule"))
	return nil
}

func (r *DaskJob) validateDaskJobName() *field.Error {
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
