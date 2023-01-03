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
	"log"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/client-go/kubernetes"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

var (
	gitOpsConfigDefaultNamespacedName types.NamespacedName = types.NamespacedName{Namespace: "openshift-operators", Name: "gitopsconfig"}
)

// GitOpsConfigReconciler reconciles a GitOpsConfig object
type GitOpsConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger

	config       *rest.Config
	configClient configclient.Interface
	fullClient   kubernetes.Interface

	olmClient olmclient.Interface
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=gitopsconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=gitopsconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=gitopsconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;get
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=list;get;create;update;patch;delete
//

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The Reconcile function compares the state specified by
// the GitOpsConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *GitOpsConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Reconcile() should perform at most one action in any invocation
	// in order to simplify testing.
	r.logger = klog.FromContext(ctx)
	r.logger.Info("Reconciling GitOpsConfig")

	// Logger includes name and namespace
	// Its also wants arguments in pairs, eg.
	// r.logger.Error(err, fmt.Sprintf("[%s/%s] %s", p.Name, p.ObjectMeta.Namespace, reason))
	// Or r.logger.Error(err, "message", "name", p.Name, "namespace", p.ObjectMeta.Namespace, "reason", reason))

	// Fetch the NodeMaintenance instance
	if !reflect.DeepEqual(gitOpsConfigDefaultNamespacedName, req.NamespacedName) {
		// not the GitOpsConfig we are looking for. This is to ensure that a single subscription is created for the openshift-gitops.
		// Any other CR that does not match the desired values will be ignored
		return ctrl.Result{}, nil
	}
	instance := &api.GitOpsConfig{}
	err := r.Client.Get(context.TODO(), gitOpsConfigDefaultNamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.logger.Info("GitOpsConfig not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.logger.Info("Error reading the request object, requeuing.")
		return reconcile.Result{}, err
	}

	// Ensure no instance has been deployed application on deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add finalizer when object is created
		if !controllerutil.ContainsFinalizer(instance, api.GitOpsConfigFinalizer) {
			controllerutil.AddFinalizer(instance, api.GitOpsConfigFinalizer)
			err := r.Client.Update(context.TODO(), instance)
			return r.actionPerformed(instance, "updated finalizer", err)
		}

	} else if err := r.finalizeObject(instance); err != nil {
		return r.actionPerformed(instance, "finalize", err)
	} else {
		log.Printf("Removing finalizer from %s\n", instance.ObjectMeta.Name)
		controllerutil.RemoveFinalizer(instance, api.GitOpsConfigFinalizer)
		if updateErr := r.Client.Status().Update(context.TODO(), instance); updateErr != nil {
			log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", "remove finalizer", err.Error())
			return reconcile.Result{}, updateErr
		}

		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "finalize")
		return reconcile.Result{}, nil
	}

	// -- Fill in defaults (changes made to a copy and not persisted)
	qualifiedInstance := r.applyGitOpsConfigDefaults(instance)

	targetSub := newSubscription(qualifiedInstance)
	err = controllerutil.SetOwnerReference(qualifiedInstance, targetSub, r.Scheme)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "unable set resource ownership", fmt.Errorf("unable to set ownership on GitOpsConfig resource"))
	}
	_, sub := getSubscription(r.olmClient, targetSub.Name, targetSub.Namespace)
	if sub == nil {
		err := createSubscription(r.olmClient, targetSub)
		return r.actionPerformed(qualifiedInstance, "create gitops subscription", err)
	}
	if ownedBySame(targetSub, sub) {
		// Check version/channel etc
		// Dangerous if multiple patterns do not agree, or automatic upgrades are in place...
		err, changed := updateSubscription(r.olmClient, targetSub, sub)
		if changed {
			return r.actionPerformed(qualifiedInstance, "update gitops subscription", err)
		}
	} else {
		logOnce("The gitops subscription is not owned by us, leaving untouched")
	}

	logOnce("subscription found")

	// -- GitOps Namespace (created by the gitops operator)
	if !haveNamespace(r.Client, applicationNamespace) {
		return r.actionPerformed(qualifiedInstance, "check application namespace", fmt.Errorf("waiting for creation"))
	}

	logOnce("namespace found")

	// Report statistics

	log.Printf("\x1b[32;1m\tReconcile complete\x1b[0m\n")

	return ctrl.Result{}, nil
}

func (r *GitOpsConfigReconciler) applyGitOpsConfigDefaults(input *api.GitOpsConfig) *api.GitOpsConfig {
	output := input.DeepCopy()

	if len(output.Spec.OperatorChannel) == 0 {
		output.Spec.OperatorChannel = "stable"
	}

	if len(output.Spec.OperatorSource) == 0 {
		output.Spec.OperatorSource = "redhat-operators"
	}
	if len(output.Spec.OperatorCSV) == 0 {
		output.Spec.OperatorCSV = "v1.4.0"
	}
	return output
}

func (r *GitOpsConfigReconciler) finalizeObject(instance *api.GitOpsConfig) error {

	// Add finalizer when object is created
	log.Printf("Finalizing pattern object")

	// The object is being deleted
	if controllerutil.ContainsFinalizer(instance, api.GitOpsConfigFinalizer) || controllerutil.ContainsFinalizer(instance, metav1.FinalizerOrphanDependents) {

		// Check that there are no patterns before deleting
		pList := api.GitOpsConfigList{}
		err := r.Client.List(context.Background(), &pList, &client.ListOptions{})
		if err != nil {
			return err
		}
		if len(pList.Items) >= 0 {
			return fmt.Errorf("unable to remove the GitOpsConfig %s in %s as not all pattern resources have been removed,", instance.Name, instance.Namespace)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitOpsConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.config = mgr.GetConfig()

	if r.configClient, err = configclient.NewForConfig(r.config); err != nil {
		return err
	}

	if r.fullClient, err = kubernetes.NewForConfig(r.config); err != nil {
		return err
	}

	if r.olmClient, err = olmclient.NewForConfig(r.config); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.GitOpsConfig{}).
		Complete(r)
}

func (r *GitOpsConfigReconciler) onReconcileErrorWithRequeue(g *api.GitOpsConfig, reason string, err error, duration *time.Duration) (reconcile.Result, error) {
	// err is logged by the reconcileHandler
	g.Status.LastStep = reason
	if err != nil {
		g.Status.LastError = err.Error()
		log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", reason, err.Error())
		//r.logger.Error(fmt.Errorf("Reconcile step failed"), reason)

	} else {
		g.Status.LastError = ""
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", reason)
	}

	updateErr := r.Client.Status().Update(context.TODO(), g)
	if updateErr != nil {
		r.logger.Error(updateErr, "Failed to update GitOpsConfig status")
	}
	if duration != nil {
		log.Printf("Requeueing\n")
		return reconcile.Result{RequeueAfter: *duration}, err
	}
	//	log.Printf("Reconciling with exponential duration")
	return reconcile.Result{}, err
}

func (r *GitOpsConfigReconciler) actionPerformed(g *api.GitOpsConfig, reason string, err error) (reconcile.Result, error) {
	if err != nil {
		delay := time.Minute
		return r.onReconcileErrorWithRequeue(g, reason, err, &delay)
	} else if !g.ObjectMeta.DeletionTimestamp.IsZero() {
		delay := time.Minute * 2
		return r.onReconcileErrorWithRequeue(g, reason, err, &delay)
	}
	return r.onReconcileErrorWithRequeue(g, reason, err, nil)
}
