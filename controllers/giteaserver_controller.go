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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8slog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// GiteaServerReconciler reconciles a GiteaServer object
type GiteaServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger
}

// var (
// 	gitea_url       = "https://charts.validatedpatterns.io/"
// 	repoName        = "helm-charts"
// 	chartName       = "gitea-chart"
// 	releaseName     = "gitea"
// 	gitea_namespace = "gitea"
// 	args            = map[string]string{}
// 	//args        = map[string]string{
// 	// comma seperated values to set
// 	//"set": "mysqlRootPassword=admin@123,persistence.enabled=false,imagePullPolicy=Always",
// 	//}
// )

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolume,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=*
//+kubebuilder:rbac:groups="route.openshift.io",resources=routes;routes/custom-host,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;replicasets;daemonsets;statefulsets,verbs=*
//+kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=*
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;create
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GiteaServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *GiteaServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = k8slog.FromContext(ctx)

	// TODO(user): your logic here
	instance := &gitopsv1alpha1.GiteaServer{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.logger.Info("GiteaServer not found")
			return ctrl.Result{}, nil
		}
	}

	// -- GitOps Namespace (created by the gitops operator)
	if !haveNamespace(r.Client, instance.Spec.Namespace) {
		fCreated, err := createNamespace(r.Client, instance.Spec.Namespace)
		if !fCreated {
			r.logger.Error(err, "GiteaServer Namespace not created.")
			return r.actionPerformed(instance, "check namespace", err)
		}
		return r.actionPerformed(instance, "check GiteaServer namespace", fmt.Errorf("waiting for creation"))
	}

	fmt.Println("Target namespace: ", instance.Spec.Namespace)

	os.Setenv("HELM_NAMESPACE", instance.Spec.Namespace)
	Init()
	fmt.Println("Calling isChartDeployed: ", instance.Spec.ReleaseName, " , ", instance.Spec.Namespace)
	if fDeployed, err := isChartDeployed(instance.Spec.ReleaseName, instance.Spec.Namespace); !fDeployed && err == nil {
		// Add helm repo
		RepoAdd(instance.Spec.RepoName, instance.Spec.HelmChartUrl)
		// Update charts from the helm repo
		RepoUpdate()
		// Install charts
		// TODO: The args are overrides for the chart
		// We need to figure out how we would pass these
		// and if we want them as part of the CRD
		args := map[string]string{}
		InstallChart(instance.Spec.ReleaseName, instance.Spec.RepoName, instance.Spec.ChartName, args)
	} else if fDeployed && err != nil {
		return r.actionPerformed(instance, "GiteaServer deployment", err)
	} else {
		fmt.Println("isChartDeployed returned: ", fDeployed, " and err: ", err)
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "GiteaServer Deploy")
	}
	var fUpdate bool
	fUpdate, err = r.updateGiteaServerCRDetails(instance)

	if err == nil && fUpdate {
		r.logger.Info("GiteaServer CR Updated")
	}
	result := ctrl.Result{
		Requeue:      false,
		RequeueAfter: ReconcileLoopRequeueTime,
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GiteaServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitopsv1alpha1.GiteaServer{}).
		Complete(r)
}

func (r *GiteaServerReconciler) onReconcileErrorWithRequeue(p *gitopsv1alpha1.GiteaServer, reason string, err error, duration *time.Duration) (reconcile.Result, error) {
	// err is logged by the reconcileHandler
	p.Status.LastStep = reason
	if err != nil {
		p.Status.LastError = err.Error()
		log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", reason, err.Error())
	} else {
		p.Status.LastError = ""
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", reason)
	}

	updateErr := r.Client.Status().Update(context.TODO(), p)
	if updateErr != nil {
		r.logger.Error(updateErr, "Failed to update GiteaServer status")
		return reconcile.Result{}, updateErr
	}
	if duration != nil {
		log.Printf("Requeueing\n")
		return reconcile.Result{RequeueAfter: *duration}, err
	}
	return reconcile.Result{}, err
}

func (r *GiteaServerReconciler) actionPerformed(p *gitopsv1alpha1.GiteaServer, reason string, err error) (reconcile.Result, error) {
	if err != nil {
		delay := time.Minute * 1
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	} else if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		delay := time.Minute * 2
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	}
	return r.onReconcileErrorWithRequeue(p, reason, err, nil)
}

// updateGiteaCRDetails updates the current GiteaServer CR Status.
// Returns true if the CR was updated else it returns false
func (r *GiteaServerReconciler) updateGiteaServerCRDetails(input *gitopsv1alpha1.GiteaServer) (bool, error) {
	rel, err := getChartRelease(input.Spec.ReleaseName, input.Spec.Namespace)

	// Return the err
	if err != nil {
		input.Status.LastStep = `update GiteaServer application status`
		input.Status.LastError = string(err.Error())
		return false, err
	}

	if input.Status.ChartStatus != string(rel.Info.Status) {
		input.Status.ChartStatus = string(rel.Info.Status)
		// Update the GiteaServer CR if difference was found
		input.Status.LastStep = `update GiteaServer application status`
		// Now let's update the CR with the application status data.
		err := r.Client.Status().Update(context.Background(), input)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
