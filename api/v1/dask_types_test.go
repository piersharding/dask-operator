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

package v1

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	// analyticsv1 "github.com/piersharding/dask-operator/api/v1"
	// "github.com/piersharding/dask-operator/models"
	// dtypes "github.com/piersharding/dask-operator/types"
)

var resource_name = "testdask"
var initialReplicas = int32(3)
var test1_name = resource_name + "3workers"

var _ = Context("Inside of a new namespace", func() {
	ctx := context.TODO()
	ns := SetupTest(ctx)
	var (
		key              types.NamespacedName
		created, fetched *Dask
	)
	Describe("when no existing resources exist", func() {

		It("should create a new Dask resource with the specified name and three worker replicas", func() {
			key = types.NamespacedName{
				Name:      test1_name,
				Namespace: ns.Name,
			}
			created = &Dask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test1_name,
					Namespace: ns.Name,
				},
				Spec: DaskSpec{
					Jupyter:          true,
					Replicas:         initialReplicas,
					Image:            "piersharding/arl-dask:latest",
					JupyterIngress:   "notebook.dask.local",
					SchedulerIngress: "scheduler.dask.local",
					MonitorIngress:   "monitor.dask.local",
					ImagePullPolicy:  "IfNotPresent",
					Env: []corev1.EnvVar{{Name: "x",
						Value: "y"}},
					Worker: &DaskDeploymentSpec{Env: []corev1.EnvVar{{Name: "x",
						Value: "y"}}},
				},
			}

			By("creating an API obj")
			Expect(k8sClient.Create(ctx, created)).To(Succeed())

			fetched = &Dask{}
			// fetched.Spec.Env[]
			Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))

			// check webhook functions
			By("validating the Dask resource attributes")
			Expect(created.ValidateCreate()).To(BeNil())
			Expect(created.ValidateUpdate(fetched)).To(BeNil())
			Expect(created.validateDaskSpec()).To(BeNil())
			// good name
			By("validating a good name")
			Expect(created.validateDaskName()).To(BeNil())
			// bad name
			By("validating a bad name")
			fetched.ObjectMeta.Name = randStringRunes(52 + 1)
			Expect(fetched.validateDaskName()).Should(HaveOccurred())
			By("Setting defaults")
			fetched.Spec.ImagePullPolicy = ""
			Expect(fetched.Spec.ImagePullPolicy).To(Equal(""))
			fetched.Default()
			Expect(fetched.Spec.ImagePullPolicy).To(Equal("IfNotPresent"))

			By("deleting the created object")
			Expect(k8sClient.Delete(ctx, created)).To(Succeed())
			Expect(k8sClient.Get(ctx, key, created)).ToNot(Succeed())

			By("Doing a Copy - deep compare")
			copy := fetched.DeepCopy()
			Expect(copy).To(BeEquivalentTo(fetched))
			// scopy := fetched.Spec.DeepCopy()
			// Expect(scopy).To(BeEquivalentTo(fetched.Spec))
			wcopy := fetched.Spec.Worker.DeepCopy()
			Expect(wcopy).To(BeEquivalentTo(fetched.Spec.Worker))
		})
	})
})
