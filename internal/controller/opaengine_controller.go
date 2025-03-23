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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opaspolimiitv1alpha1 "github.com/bramba2000/opa-scaler/api/v1alpha1"
	opamanager "github.com/bramba2000/opa-scaler/internal/opa"
)

const (
	// typeAvailableOpaEngine is the type of the condition for an OpaEngine that is available
	typeAvailableOpaEngine = "Available"
	// typeProgressingOpaEngine is the type of the condition for an OpaEngine that is progressing
	typeProgressingOpaEngine = "Progressing"
	// typeDegradedOpaEngine is the type of the condition for an OpaEngine that is degraded
	typeDegradedOpaEngine = "Degraded"
)

const OpaEngineFinalizer = "opa-scaler.polimi.it/oe-finalizer"

// OpaEngineReconciler reconciles a OpaEngine object
type OpaEngineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=opas.polimi.it,resources=opaengines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opas.polimi.it,resources=opaengines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opas.polimi.it,resources=opaengines/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a OpaEngine object and makes changes based on the state read
// and what is in the OpaEngine.Spec
func (r *OpaEngineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpaEngine instance
	engine := &opaspolimiitv1alpha1.OpaEngine{}
	if err := r.Get(ctx, req.NamespacedName, engine); err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			logger.Error(err, "unable to fetch OpaEngine")
		}
		return ctrl.Result{}, err
	}

	// Check if never reconciled
	if len(engine.Status.Conditions) == 0 {
		meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:    typeProgressingOpaEngine,
			Status:  metav1.ConditionUnknown,
			Reason:  "Reconciling",
			Message: "Starting reconciliation of the OpaEngine",
		})
		if err := r.Status().Update(ctx, engine); err != nil {
			logger.Error(err, "unable to update OpaEngine status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, engine); err != nil {
			logger.Error(err, "unable to fetch OpaEngine")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(engine, OpaEngineFinalizer) {
		logger.Info("Adding finalizer to OpaEngine")
		controllerutil.AddFinalizer(engine, OpaEngineFinalizer)
		if err := r.Update(ctx, engine); err != nil {
			logger.Error(err, "unable to add finalizer to OpaEngine")
			return ctrl.Result{}, err
		}
	}

	// Check if marked to be deleted
	if engine.GetDeletionTimestamp() != nil {
		// Signal that the OpaEngine is being deleted
		meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:    typeDegradedOpaEngine,
			Status:  metav1.ConditionUnknown,
			Reason:  "Finalizing",
			Message: fmt.Sprintf("OpaEngine %s is being deleted", engine.Name),
		})

		if err := r.Status().Update(ctx, engine); err != nil {
			logger.Error(err, "unable to update OpaEngine status")
			return ctrl.Result{}, err
		}

		// TODO(user): Add the finalizer logic if needed (and re-fetch it)

		// Signal that the OpaEngine finalizing work is completed
		meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:    typeDegradedOpaEngine,
			Status:  metav1.ConditionTrue,
			Reason:  "Finalizing",
			Message: "OpaEngine finalizer completed",
		})
		if err := r.Status().Update(ctx, engine); err != nil {
			logger.Error(err, "unable to update OpaEngine status")
			return ctrl.Result{}, err
		}

		// Remove the finalizer
		controllerutil.RemoveFinalizer(engine, OpaEngineFinalizer)
		if err := r.Update(ctx, engine); err != nil {
			logger.Error(err, "unable to remove finalizer from OpaEngine")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Check the OpaEngine service
	foundService := &corev1.Service{}
	err := r.Get(ctx, req.NamespacedName, foundService)

	// If the service doesn't exist, create it
	if err != nil && apierrors.IsNotFound(err) {
		// Create the service
		ser, err := r.serviceForOpaEngine(engine)
		if err != nil {
			logger.Error(err, "unable to create service for OpaEngine")

			meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
				Type:    typeDegradedOpaEngine,
				Status:  metav1.ConditionTrue,
				Reason:  "ServiceError",
				Message: "Unable to create Service for OpaEngine",
			})

			if err := r.Status().Update(ctx, engine); err != nil {
				logger.Error(err, "unable to update OpaEngine status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		// If the service owner has been register, create it
		logger.Info("Creating a new Service", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.Create(ctx, ser)
		if err != nil {
			logger.Error(err, "unable to create Service for OpaEngine", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return ctrl.Result{}, err
		}
	}

	// Check the OpaEngine deployment
	foundDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, foundDeployment)

	// If the deployment doesn't exist, create it
	if err != nil && apierrors.IsNotFound(err) {
		// Create the deployment
		dep, err := r.deploymentForOpaEngine(engine)
		if err != nil {
			// The error has been thrown only if there is another OwnerReference with Controller flag set
			logger.Error(err, "unable to create deployment for OpaEngine")

			meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
				Type:    typeDegradedOpaEngine,
				Status:  metav1.ConditionTrue,
				Reason:  "DeploymentError",
				Message: "Unable to create Deployment for OpaEngine",
			})

			if err := r.Status().Update(ctx, engine); err != nil {
				logger.Error(err, "unable to update OpaEngine status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		// If the deployment owner has been register, create it
		logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			logger.Error(err, "unable to create Deployment for OpaEngine", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully - return and requeue
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	} else if err != nil {
		logger.Error(err, "unable to get Deployment for OpaEngine")
		return ctrl.Result{}, err
	}

	// Check if all conditions are satisfied
	if foundDeployment.Status.AvailableReplicas == *foundDeployment.Spec.Replicas {
		meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:    typeAvailableOpaEngine,
			Status:  metav1.ConditionTrue,
			Reason:  "Available",
			Message: "OpaEngine is available",
		})
	} else {
		meta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:    typeAvailableOpaEngine,
			Status:  metav1.ConditionFalse,
			Reason:  "Unavailable",
			Message: "OpaEngine is not available",
		})
	}

	if err := r.Status().Update(ctx, engine); err != nil {
		logger.Error(err, "unable to update OpaEngine status")
		return ctrl.Result{}, err
	}

	// If the deployment is available, process policies
	if foundDeployment.Status.AvailableReplicas == *foundDeployment.Spec.Replicas {
		toBeAdded, _ := opamanager.MergePolicies(engine.Spec.Policies, engine.Status.Policies)
		url := fmt.Sprintf("http://%s.%s.svc.cluster.local:8181", engine.Name, engine.Namespace)
		if len(toBeAdded) > 0 {
			policies := map[string]string{
				toBeAdded[0]: "package test\n\ndefault allow = false\n",
			}
			logger.Info("Adding policies", "Policies", toBeAdded)
			added, err := opamanager.PushPolicies(ctx, url, policies)
			if err != nil {
				logger.Error(err, "unable to add policies")
				return ctrl.Result{}, err
			}
			engine.Status.Policies = append(engine.Status.Policies, added...)
			if err := r.Status().Update(ctx, engine); err != nil {
				logger.Error(err, "unable to update OpaEngine status")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpaEngineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opaspolimiitv1alpha1.OpaEngine{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// Generate the deployment for the OpaEngine
func (r *OpaEngineReconciler) deploymentForOpaEngine(engine *opaspolimiitv1alpha1.OpaEngine) (*appsv1.Deployment, error) {
	labels := map[string]string{
		"app.kubernetes.io/name":       engine.Name,
		"app.kubernetes.io/instance":   engine.Spec.InstanceName,
		"app.kubernetes.io/component":  "opa-engine",
		"app.kubernetes.io/part-of":    "opa-scaler",
		"app.kubernetes.io/managed-by": "opa-scaler-operator",
	}

	replicas := engine.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      engine.Name,
			Namespace: engine.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "opa",
							Image: engine.Spec.Image,
							Args:  []string{"run", "--server", "--addr", ":8181", "--log-level", "debug"},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(8181),
										Scheme: corev1.URISchemeHTTP,
									}},
								InitialDelaySeconds: 2,
								PeriodSeconds:       3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health?bundle=true",
										Port:   intstr.FromInt(8181),
										Scheme: corev1.URISchemeHTTP,
									}},
								InitialDelaySeconds: 2,
								PeriodSeconds:       3,
							},
						},
					},
				},
			},
		},
	}

	// Set OpaEngine instance as the owner and controller
	if err := ctrl.SetControllerReference(engine, dep, r.Scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

// Generate the service for the OpaEngine
func (r *OpaEngineReconciler) serviceForOpaEngine(engine *opaspolimiitv1alpha1.OpaEngine) (*corev1.Service, error) {
	labels := map[string]string{
		"app.kubernetes.io/name":       engine.Name,
		"app.kubernetes.io/instance":   engine.Spec.InstanceName,
		"app.kubernetes.io/component":  "opa-engine",
		"app.kubernetes.io/part-of":    "opa-scaler",
		"app.kubernetes.io/managed-by": "opa-scaler-operator",
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      engine.Name,
			Namespace: engine.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8181,
				},
			},
		},
	}

	// Set OpaEngine instance as the owner and controller
	if err := ctrl.SetControllerReference(engine, svc, r.Scheme); err != nil {
		return nil, err
	}

	return svc, nil
}
