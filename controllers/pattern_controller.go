/*
Copyright 2022.

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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

// PatternReconciler reconciles a Pattern object
type PatternReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The Reconcile function compares the state specified by
// the Pattern object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PatternReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Reconcile() should perform at most one action in any invocation
	// in order to simplify testing.
	r.logger = log.FromContext(ctx)
	r.logger.Info("Reconciling NodeMaintenance")

	// Fetch the NodeMaintenance instance
	instance := &api.Pattern{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.logger.Info("Pattern not found", "name", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.logger.Info("Error reading the request object, requeuing.")
		return reconcile.Result{}, err
	}

	// Add finalizer when object is created
	// 	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
	// 		if !ContainsString(instance.ObjectMeta.Finalizers, nodemaintenancev1beta1.NodeMaintenanceFinalizer) {
	// 			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, nodemaintenancev1beta1.NodeMaintenanceFinalizer)
	// 			if err := r.Client.Update(context.TODO(), instance); err != nil {
	// 				return r.onReconcileError(instance, err)
	// 			}
	// 		}
	// 	} else {
	// 		r.logger.Info("Deletion timestamp not zero")
	//
	// 		// The object is being deleted
	// 		if ContainsString(instance.ObjectMeta.Finalizers, nodemaintenancev1beta1.NodeMaintenanceFinalizer) || ContainsString(instance.ObjectMeta.Finalizers, metav1.FinalizerOrphanDependents) {
	// 			// Stop node maintenance - uncordon and remove live migration taint from the node.
	// 			if err := r.stopNodeMaintenanceOnDeletion(instance.Spec.NodeName); err != nil {
	// 				r.logger.Error(err, "error stopping node maintenance")
	// 				if errors.IsNotFound(err) == false {
	// 					return r.onReconcileError(instance, err)
	// 				}
	// 			}
	//
	// 			// Remove our finalizer from the list and update it.
	// 			instance.ObjectMeta.Finalizers = RemoveString(instance.ObjectMeta.Finalizers, nodemaintenancev1beta1.NodeMaintenanceFinalizer)
	// 			if err := r.Client.Update(context.Background(), instance); err != nil {
	// 				return r.onReconcileError(instance, err)
	// 			}
	// 		}
	// 		return reconcile.Result{}, nil
	// 	}

	// Fill in defaults - Make changes in the copy?
	err, qualifiedInstance := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	// Update/create the gitops subscription

	// Update/create the argo application

	chart := chartForPattern(*qualifiedInstance)

	if chart == nil && len(qualifiedInstance.Status.Path) == 0 {
		err := r.Prepare(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "cloning pattern repo", err)
	}

	if chart == nil {
		err := r.Deploy(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "deploying the pattern", err)
	}

	// Reconcile any changes

	// Perform validation of the site values file(s)
	// Report statistics

	return r.actionPerformed(qualifiedInstance, "done", nil)
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (error, *api.Pattern) {
	return nil, input
}

func (r *PatternReconciler) Prepare(p *api.Pattern) error {
	if len(p.Status.Path) > 0 {
		volumePath := "/"
		unique := uuid.New().URN()
		p.Status.Path = fmt.Sprintf("%s/%s/%s/%s", volumePath, p.Namespace, p.Name, unique)
	}

	// if directory != exists...
	err := checkout(p.Spec.GitSpec.TargetRepo, p.Status.Path, &p.Spec.GitSpec.Token, &p.Spec.GitSpec.TargetRevision)
	return err
}

func (r *PatternReconciler) Deploy(p *api.Pattern) error {
	sampleValues := map[string]interface{}{
		"redis": map[string]interface{}{
			"sentinel": map[string]interface{}{
				"masterName": "BigMaster",
				"pass":       "random",
				"addr":       "localhost",
				"port":       "26379",
			},
		},
	}

	chart := HelmChart{
		Name:       p.Name,
		Namespace:  p.ObjectMeta.Namespace,
		Version:    0,
		Path:       p.Status.Path,
		Parameters: sampleValues,
	}

	err, version := installChart(chart)
	if err == nil {
		r.logger.Info("Deployed %s/%s: %d.", p.Name, p.ObjectMeta.Namespace, version)
	} else {
		p.Status.Version = version
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PatternReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Pattern{}).
		Complete(r)
}

func (r *PatternReconciler) onReconcileErrorWithRequeue(p *api.Pattern, reason string, err error, duration *time.Duration) (reconcile.Result, error) {
	p.Status.LastError = err.Error()
	r.logger.Error(err, "[%s/%s] %s", p.Name, p.ObjectMeta.Namespace, reason)

	updateErr := r.Client.Status().Update(context.TODO(), p)
	if updateErr != nil {
		r.logger.Error(updateErr, "Failed to update Pattern status")
	}
	//		return reconcile.Result{RequeueAfter: *duration}, nil
	r.logger.Info("Reconciling with exponential duration")
	return reconcile.Result{}, err
}

func (r *PatternReconciler) actionPerformed(p *api.Pattern, reason string, err error) (reconcile.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	return r.onReconcileErrorWithRequeue(p, reason, err, nil)
}
