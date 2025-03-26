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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opaspolimiitv1alpha1 "github.com/bramba2000/opa-scaler/api/v1alpha1"
)

// DependencyReconciler reconciles a Dependency object
type DependencyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=opas.polimi.it,resources=dependencies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opas.polimi.it,resources=dependencies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opas.polimi.it,resources=dependencies/finalizers,verbs=update
// +kubebuilder:rbac:groups=opas.polimi.it,resources=policies,verbs=get;list;watch
// +kubebuilder:rbac:groups=opas.polimi.it,resources=opaengine,verbs=get;list;watch;create;update;patch;delete

func (r *DependencyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Dependency instance
	depCR := &opaspolimiitv1alpha1.Dependency{}
	if err := r.Get(ctx, req.NamespacedName, depCR); err != nil {
		logger.Error(err, "unable to fetch Dependency")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// if no conditions are set, set the default ones
	if len(depCR.Status.Conditions) == 0 {
		if err := r.addCondition(ctx, req, metav1.Condition{
			Type:    "Available",
			Status:  metav1.ConditionUnknown,
			Reason:  "DependencyNotReady",
			Message: "Dependency is not ready",
		}); err != nil {
			logger.Error(err, "unable to set default conditions")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		}
		logger.Info("Default conditions set")
	}

	// Check if the dependency is already deployed
	if depCR.Status.Deployed {
		logger.Info("Dependency already deployed")
		return ctrl.Result{}, nil
	}

	// Fetch the policy instance
	policyCR := &opaspolimiitv1alpha1.Policy{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      depCR.Spec.PolicyName,
	}, policyCR); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Policy not found - set the condition
			r.addCondition(ctx, req, metav1.Condition{
				Type:    "Available",
				Status:  metav1.ConditionFalse,
				Reason:  "PolicyNotFound",
				Message: "Policy not found",
			})
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		} else {
			logger.Error(err, "unable to fetch Policy")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		}
	}

	// Check if there is a policy engine
	engines := new(opaspolimiitv1alpha1.OpaEngineList)
	opts := []client.ListOption{
		client.InNamespace(req.Namespace),
	}
	if err := r.List(ctx, engines, opts...); err != nil {
		// If error is not found, create a new engine
		logger.Error(err, "unable to fetch OpaEngine")

	} else if len(engines.Items) == 0 {
		// No engine found, create it
		newEngine := &opaspolimiitv1alpha1.OpaEngine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: req.Namespace,
			},
			Spec: opaspolimiitv1alpha1.OpaEngineSpec{
				InstanceName: "default",
			},
		}
		if res, err := controllerutil.CreateOrUpdate(ctx, r.Client, newEngine, func() error {
			if newEngine.ObjectMeta.CreationTimestamp.IsZero() {
				newEngine.Spec.Policies = []string{depCR.Spec.PolicyName}
			} else {
				newEngine.Spec.Policies = append(newEngine.Spec.Policies, depCR.Spec.PolicyName)
			}
			return nil
		}); err != nil {
			logger.Error(err, "unable to create OpaEngine")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		} else if res != controllerutil.OperationResultNone {
			logger.Info("OpaEngine created")
			// Set the condition
			if err := r.addCondition(ctx, req, metav1.Condition{
				Type:    "Available",
				Status:  metav1.ConditionFalse,
				Reason:  "Scheduled",
				Message: "Dependency scheduled in default engine",
			}); err != nil {
				logger.Error(err, "unable to set condition")
				return ctrl.Result{RequeueAfter: 1 * time.Second}, err
			}
			// Update the status
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				if err := r.Get(ctx, req.NamespacedName, depCR); err != nil {
					return err
				}
				depCR.Status.EngineName = append(depCR.Status.EngineName, newEngine.Name)
				return r.Status().Update(ctx, depCR)
			}); err != nil {
				logger.Error(err, "unable to update status")
				return ctrl.Result{RequeueAfter: 1 * time.Second}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opaspolimiitv1alpha1.Dependency{}).
		Complete(r)
}

func (r *DependencyReconciler) addCondition(ctx context.Context, req ctrl.Request, newCondition metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		depCR := new(opaspolimiitv1alpha1.Dependency)
		if err := r.Get(ctx, req.NamespacedName, depCR); err != nil {
			return err
		}

		if changed := meta.SetStatusCondition(&depCR.Status.Conditions, newCondition); !changed {
			return nil
		}
		return r.Status().Update(ctx, depCR)
	})
}

func (r *DependencyReconciler) addPolicyToEngine(ctx context.Context, policyName string, engine *opaspolimiitv1alpha1.OpaEngine) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if engine.Spec.Policies == nil {
			engine.Spec.Policies = []string{policyName}
		} else {
			engine.Spec.Policies = append(engine.Spec.Policies, policyName)
		}
		return r.Update(ctx, engine)
	})
}
