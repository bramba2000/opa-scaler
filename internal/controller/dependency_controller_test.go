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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opaspolimiitv1alpha1 "github.com/bramba2000/opa-scaler/api/v1alpha1"
)

var _ = Describe("Dependency Controller", Focus, func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		dependency := &opaspolimiitv1alpha1.Dependency{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Dependency")
			err := k8sClient.Get(ctx, typeNamespacedName, dependency)
			if err != nil && errors.IsNotFound(err) {
				resource := &opaspolimiitv1alpha1.Dependency{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: opaspolimiitv1alpha1.DependencySpec{
						ServiceName: "test-service",
						PolicyName:  "test-policy",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &opaspolimiitv1alpha1.Dependency{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Dependency")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should mark the resource as unavailable when no policy is found", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DependencyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			dependency := new(opaspolimiitv1alpha1.Dependency)
			Expect(k8sClient.Get(ctx, typeNamespacedName, dependency)).To(Succeed())
			Expect(dependency.Status.Conditions).To(HaveLen(1))
			Expect(dependency.Status.Conditions[0].Type).To(Equal("Available"))
			Expect(dependency.Status.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(dependency.Status.Conditions[0].Reason).To(Equal("PolicyNotFound"))
		})

		It("should mark the resource as available when policy engine is found", func() {
			By("Creating the policy")
			policy := &opaspolimiitv1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: opaspolimiitv1alpha1.PolicySpec{
					Rego: `package test
					default allow = false`,
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
			By("Reconciling the created resource")
			controllerReconciler := &DependencyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			engine := new(opaspolimiitv1alpha1.OpaEngine)
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "default-engine",
				Namespace: typeNamespacedName.Namespace,
			}, engine)).To(Succeed())

			By("reconciling the resource again")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			dependency := new(opaspolimiitv1alpha1.Dependency)
			Expect(k8sClient.Get(ctx, typeNamespacedName, dependency)).To(Succeed())
			Expect(dependency.Status.Conditions).To(HaveLen(1))
			Expect(dependency.Status.Conditions[0].Type).To(Equal("Available"))
			Expect(dependency.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(dependency.Status.Conditions[0].Reason).To(Equal("PolicyAdded"))
			Expect(dependency.Status.Conditions[0].Message).To(ContainSubstring("default"))

			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "default-engine",
				Namespace: typeNamespacedName.Namespace,
			}, engine)).To(Succeed())
			Expect(engine.Spec.Policies).To(ContainElement(policy.Name))
		})
	})
})
