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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	analyticsv1 "gitlab.com/piersharding/dask-operator/api/v1"
	// "gitlab.com/piersharding/dask-operator/models"
	// dtypes "gitlab.com/piersharding/dask-operator/types"
)

// var resource_name = "testdask"
// var initialReplicas = int32(3)
// var test1_name = resource_name + "3workers"
// var test2_name = resource_name + "defaultworkers"

var _ = Context("Inside of a new namespace", func() {
	ctx := context.TODO()
	ns := SetupTest(ctx)

	Describe("when no existing resources exist", func() {

		It("should create a new DaskJob eventually", func() {

			daskjob := &analyticsv1.DaskJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test1_name,
					Namespace: ns.Name,
				},
				Spec: analyticsv1.DaskJobSpec{
					Report:  true,
					Cluster: test1_name,
					Image:   "jupyter/scipy-notebook:latest",
					Script:  "https://raw.githubusercontent.com/piersharding/dask-operator/master/notebooks/array.ipynb",
				},
			}

			err := k8sClient.Create(ctx, daskjob)
			Expect(err).NotTo(HaveOccurred(), "failed to create test DaskJob resource")

			dask := &analyticsv1.Dask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test1_name,
					Namespace: ns.Name,
				},
				Spec: analyticsv1.DaskSpec{
					Jupyter:          true,
					Replicas:         initialReplicas,
					Image:            "daskdev/dask:2.9.0",
					JupyterIngress:   "notebook.dask.local",
					SchedulerIngress: "scheduler.dask.local",
					MonitorIngress:   "monitor.dask.local",
					ImagePullPolicy:  "Always",
				},
			}

			err = k8sClient.Create(ctx, dask)
			Expect(err).NotTo(HaveOccurred(), "failed to create test Dask resource")

			// check the jupyter,scheduler, and worker deployments
			deployment := &apps.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "dask-scheduler-" + test1_name, Namespace: dask.Namespace}, deployment),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))

			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "dask-worker-" + test1_name, Namespace: dask.Namespace}, deployment),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(*deployment.Spec.Replicas).To(Equal(initialReplicas))

			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "jupyter-notebook-" + test1_name, Namespace: dask.Namespace}, deployment),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
		})

		It("should create a new Dask resource with the specified name and five worker replicas if none are specified", func() {
			dask := &analyticsv1.Dask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test2_name,
					Namespace: ns.Name,
				},
				Spec: analyticsv1.DaskSpec{
					Jupyter:          true,
					Image:            "piersharding/arl-dask:latest",
					JupyterIngress:   "notebook.dask.local",
					SchedulerIngress: "scheduler.dask.local",
					MonitorIngress:   "monitor.dask.local",
					ImagePullPolicy:  "IfNotPresent",
				},
			}

			err := k8sClient.Create(ctx, dask)
			Expect(err).NotTo(HaveOccurred(), "failed to create test Dask resource")

			deployment := &apps.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "dask-worker-" + test2_name, Namespace: dask.Namespace}, deployment),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(*deployment.Spec.Replicas).To(Equal(int32(5)))
		})

		It("should allow updating the replicas count after creating a Dask resource", func() {
			daskObjectKey := client.ObjectKey{
				Name:      test2_name,
				Namespace: ns.Name,
			}
			dask := &analyticsv1.Dask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test2_name,
					Namespace: ns.Name,
				},
				Spec: analyticsv1.DaskSpec{
					Jupyter:          true,
					Image:            "piersharding/arl-dask:latest",
					JupyterIngress:   "notebook.dask.local",
					SchedulerIngress: "scheduler.dask.local",
					MonitorIngress:   "monitor.dask.local",
					ImagePullPolicy:  "IfNotPresent",
				},
			}

			err := k8sClient.Create(ctx, dask)
			Expect(err).NotTo(HaveOccurred(), "failed to create test Dask resource")
			// daskObjectKey := client.ObjectKey{
			// 	Name:      "testresource",
			// 	Namespace: ns.Name,
			// }
			// dask := &analyticsv1.Dask{
			// 	ObjectMeta: metav1.ObjectMeta{
			// 		Name:      daskObjectKey.Name,
			// 		Namespace: daskObjectKey.Namespace,
			// 	},
			// 	Spec: analyticsv1.DaskSpec{
			// 		DeploymentName: daskObjectKey.Name,
			// 	},
			// }

			// var dask analyticsv1.Dask
			// if err := r.Get(ctx, req.NamespacedName, &dask); err != nil {

			// err := k8sClient.Create(ctx, dask)
			// Expect(err).NotTo(HaveOccurred(), "failed to create test Dask resource")

			Eventually(
				getResourceFunc(ctx, daskObjectKey, dask),
				time.Second*5, time.Millisecond*500).Should(BeNil(), "dask resource should exist")

			deployment := &apps.Deployment{}
			Eventually(
				getResourceFunc(ctx, client.ObjectKey{Name: "dask-worker-" + test2_name, Namespace: dask.Namespace}, deployment),
				time.Second*5, time.Millisecond*500).Should(BeNil())

			Expect(*deployment.Spec.Replicas).To(Equal(int32(5)), "replica count should be equal to 5")

			err = k8sClient.Get(ctx, daskObjectKey, dask)
			Expect(err).NotTo(HaveOccurred(), "failed to retrieve Dask resource")

			dask.Spec.Replicas = int32(2)
			err = k8sClient.Update(ctx, dask)
			Expect(err).NotTo(HaveOccurred(), "failed to Update Dask resource")
			deploymentObjectKey := client.ObjectKey{
				Name:      "dask-worker-" + test2_name,
				Namespace: dask.Namespace,
			}
			Eventually(getDeploymentReplicasFunc(ctx, deploymentObjectKey),
				time.Second*5, time.Millisecond*500).
				Should(Equal(int32(2)), "expected Worker Deployment resource to be scale to 2 replicas")
		})

		// It("should clean up an old Deployment resource if the deploymentName is changed", func() {
		// 	deploymentObjectKey := client.ObjectKey{
		// 		Name:      "deployment-name",
		// 		Namespace: ns.Name,
		// 	}
		// 	newDeploymentObjectKey := client.ObjectKey{
		// 		Name:      "new-deployment",
		// 		Namespace: ns.Name,
		// 	}
		// 	daskObjectKey := client.ObjectKey{
		// 		Name:      "testresource",
		// 		Namespace: ns.Name,
		// 	}
		// 	dask := &analyticsv1.Dask{
		// 		ObjectMeta: metav1.ObjectMeta{
		// 			Name:      daskObjectKey.Name,
		// 			Namespace: daskObjectKey.Namespace,
		// 		},
		// 		Spec: analyticsv1.DaskSpec{
		// 			DeploymentName: deploymentObjectKey.Name,
		// 		},
		// 	}

		// 	err := k8sClient.Create(ctx, dask)
		// 	Expect(err).NotTo(HaveOccurred(), "failed to create test Dask resource")

		// 	deployment := &apps.Deployment{}
		// 	Eventually(
		// 		getResourceFunc(ctx, deploymentObjectKey, deployment),
		// 		time.Second*5, time.Millisecond*500).Should(BeNil(), "deployment resource should exist")

		// 	err = k8sClient.Get(ctx, daskObjectKey, dask)
		// 	Expect(err).NotTo(HaveOccurred(), "failed to retrieve Dask resource")

		// 	dask.Spec.DeploymentName = newDeploymentObjectKey.Name
		// 	err = k8sClient.Update(ctx, dask)
		// 	Expect(err).NotTo(HaveOccurred(), "failed to Update Dask resource")

		// 	Eventually(
		// 		getResourceFunc(ctx, deploymentObjectKey, deployment),
		// 		time.Second*5, time.Millisecond*500).ShouldNot(BeNil(), "old deployment resource should be deleted")

		// 	Eventually(
		// 		getResourceFunc(ctx, newDeploymentObjectKey, deployment),
		// 		time.Second*5, time.Millisecond*500).Should(BeNil(), "new deployment resource should be created")
		// })
	})
})
