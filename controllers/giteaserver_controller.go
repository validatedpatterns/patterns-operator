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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

// GiteaServerReconciler reconciles a GiteaServer object
type GiteaServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	gitea_url       = "https://charts.validatedpatterns.io/"
	repoName        = "helm-charts"
	chartName       = "gitea-chart"
	releaseName     = "gitea"
	gitea_namespace = "gitea"
	args            = map[string]string{}
	//args        = map[string]string{
	// comma seperated values to set
	//"set": "mysqlRootPassword=admin@123,persistence.enabled=false,imagePullPolicy=Always",
	//}
)

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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	os.Setenv("HELM_NAMESPACE", gitea_namespace)
	Init()
	// Add helm repo
	RepoAdd(repoName, gitea_url)
	// Update charts from the helm repo
	RepoUpdate()
	// Install charts
	InstallChart(releaseName, repoName, chartName, args)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GiteaServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitopsv1alpha1.GiteaServer{}).
		Complete(r)
}
