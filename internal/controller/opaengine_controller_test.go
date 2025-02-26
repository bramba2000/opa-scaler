/*
Copyright 2025 Matteo Brambilla <matteo15.brambilla@polimi.it>.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opaspolimiitv1alpha1 "github.com/bramba2000/opa-scaler/api/v1alpha1"
)

var _ = Describe("OpaEngine Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		opaengine := &opaspolimiitv1alpha1.OpaEngine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind OpaEngine")
			err := k8sClient.Get(ctx, typeNamespacedName, opaengine)
			if err != nil && errors.IsNotFound(err) {
				resource := &opaspolimiitv1alpha1.OpaEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: opaspolimiitv1alpha1.OpaEngineSpec{
						InstanceName: "default",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &opaspolimiitv1alpha1.OpaEngine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if errors.IsNotFound(err) {
				Skip("Resource already deleted")
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance OpaEngine")
			resource.SetFinalizers([]string{})
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully set defaults", func() {
			resource := &opaspolimiitv1alpha1.OpaEngine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Spec.Replicas).To(Equal(int32(1)))
			Expect(resource.Spec.Image).To(Equal("openpolicyagent/opa:latest-envoy"))
		})

		It("should successfully set initial condition", func() {
			By("Checking the status of the OpaEngine before reconciliation")
			err := k8sClient.Get(ctx, typeNamespacedName, opaengine)
			Expect(opaengine.Status.Conditions).To(HaveLen(0))
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the OpaEngine")
			controllerReconciler := &OpaEngineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the status of the OpaEngine after reconciliation")
			err = k8sClient.Get(ctx, typeNamespacedName, opaengine)
			Expect(err).NotTo(HaveOccurred())
			Expect(opaengine.Status.Conditions).To(HaveLen(1))
			Expect(opaengine.Status.Conditions[0].Type).To(Equal("Progressing"))
			Expect(opaengine.Status.Conditions[0].Status).To(Equal(metav1.ConditionUnknown))
			Expect(opaengine.Status.Conditions[0].Reason).To(Equal("Reconciling"))
			Expect(opaengine.Status.Conditions[0].Message).To(Equal("Starting reconciliation of the OpaEngine"))
		})

		It("should successfully create owned resources", func() {
			By("Reconciling the OpaEngine")
			controllerReconciler := &OpaEngineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the deployment created")
			deployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.OwnerReferences).To(HaveLen(1))

			By("Checking the service created")
			service := &corev1.Service{}
			err = k8sClient.Get(ctx, typeNamespacedName, service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.OwnerReferences).To(HaveLen(1))
		})

		It("should successfully add finalizer", func() {
			By("Reconciling the OpaEngine")
			controllerReconciler := &OpaEngineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the finalizer added")
			err = k8sClient.Get(ctx, typeNamespacedName, opaengine)
			Expect(err).NotTo(HaveOccurred())
			Expect(opaengine.Finalizers).To(ContainElement("opa-scaler.polimi.it/oe-finalizer"))
		})

		It("should successfully delete resource", func() {
			By("Reconciling the OpaEngine")
			controllerReconciler := &OpaEngineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, opaengine)).To(Succeed())

			By("Checking the deployment created")
			deployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, deployment)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance OpaEngine")
			Expect(k8sClient.Get(ctx, typeNamespacedName, opaengine)).To(Succeed())
			Expect(k8sClient.Delete(ctx, opaengine)).To(Succeed())

			By("Triggering again the reconciliation")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, typeNamespacedName, opaengine)
				return err
			}).Should(HaveOccurred())
		})

	})
})
