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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GiteaServerReconciler reconciles a GiteaServer object
type GiteaServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger
}

// RBAC rules for the Gitea controller
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=giteaservers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolume,verbs=watch;list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=watch;list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=watch;list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=*
//+kubebuilder:rbac:groups="route.openshift.io",resources=routes;routes/custom-host,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;replicasets;daemonsets;statefulsets,verbs=*
//+kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=*
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=list;watch;delete;update;get;create;patch
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

	// Get a GiteaServer instance if exists
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
	// Fill in the defaults if needed
	// TODO: Follow example in patterns_controller
	if instance.Spec.ReleaseName == "" {
		instance.Spec.ReleaseName = ReleaseName
	}
	if instance.Spec.RepoName == "" {
		instance.Spec.RepoName = RepoName
	}
	if instance.Spec.ChartName == "" {
		instance.Spec.ChartName = ChartName
	}
	if instance.Spec.Namespace == "" {
		instance.Spec.Namespace = Gitea_Namespace
	}
	if instance.Spec.HelmChartUrl == "" {
		instance.Spec.HelmChartUrl = Helm_Chart_Repo_URL
	}

	fmt.Println("Instance: ", instance)

	// Remove the Chart on deletion
	// TODO: Make this a util function?
	//nolint:dupl
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add finalizer when object is created
		if !controllerutil.ContainsFinalizer(instance, gitopsv1alpha1.GiteaServerFinalizer) {
			controllerutil.AddFinalizer(instance, gitopsv1alpha1.GiteaServerFinalizer)
			err = r.Client.Update(context.TODO(), instance)
			return r.actionPerformed(instance, "updated finalizer", err)
		}
	} else if err = r.finalizeObject(instance); err != nil {
		return r.actionPerformed(instance, "finalize", err)
	} else {
		log.Printf("Removing finalizer from %s\n", instance.ObjectMeta.Name)
		controllerutil.RemoveFinalizer(instance, gitopsv1alpha1.GiteaServerFinalizer)
		if err = r.Client.Update(context.TODO(), instance); err != nil {
			log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", "remove finalizer", err.Error())
			return reconcile.Result{}, err
		}
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "finalize")
		return reconcile.Result{}, nil
	}

	// -- Gitea Namespace (created if it is not found)
	if !haveNamespace(r.Client, instance.Spec.Namespace) {
		fCreated, err := createNamespace(r.Client, instance.Spec.Namespace) //nolint:govet
		if !fCreated {
			r.logger.Error(err, "GiteaServer Namespace not created.")
			return r.actionPerformed(instance, "check namespace", err)
		}
		return r.actionPerformed(instance, "check GiteaServer namespace", fmt.Errorf("waiting for creation"))
	}

	os.Setenv("HELM_NAMESPACE", instance.Spec.Namespace)
	// Initialiaze Helm settings
	Init()

	// See if chart has been deployed.
	var fDeployed bool
	if fDeployed, err = isChartDeployed(instance.Spec.ReleaseName, instance.Spec.Namespace); !fDeployed && err == nil {
		// Add helm repo
		_, err = RepoAdd(instance.Spec.RepoName, instance.Spec.HelmChartUrl)
		if err != nil {
			return r.actionPerformed(instance, "add helm repo", err)
		}
		// Update charts from the helm repo
		_, err = RepoUpdate()
		if err != nil {
			return r.actionPerformed(instance, "update helm repo", err)
		}
		// Install charts
		// TODO: The args are overrides for the chart
		// We need to figure out how we would pass these
		// and if we want them as part of the CRD
		args := map[string]string{}
		_, err = InstallChart(instance.Spec.ReleaseName, instance.Spec.RepoName, instance.Spec.ChartName, args)
		if err != nil {
			return r.actionPerformed(instance, "install helm chart", err)
		}
	} else if fDeployed && err != nil {
		return r.actionPerformed(instance, "GiteaServer deployment", err)
	} else {
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "GiteaServer Deploy")
	}

	// Updated the GiteaServer CR if necessary
	var fUpdate bool
	if fUpdate, err = r.updateGiteaServerCRDetails(instance); err == nil && fUpdate {
		r.logger.Info("GiteaServer CR Updated")
	}

	// Reset the reconcile loop to get called in 180 seconds
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

//nolint:dupl
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

//nolint:dupl
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
	fUpdateCR := false
	rel, err := getChartRelease(input.Spec.ReleaseName, input.Spec.Namespace)
	input.Status.LastStep = `update GiteaServer CR status`

	// Return the err
	if err != nil {
		input.Status.LastError = err.Error()
		return false, err
	}

	// Compare the helm release info. Change status info if needed.
	if input.Status.ChartStatus != string(rel.Info.Status) {
		input.Status.ChartStatus = string(rel.Info.Status)
		fUpdateCR = true
	}

	url, err := getRoute(r.Client, "gitea-route", input.Spec.Namespace)
	if err == nil && input.Status.Route != url {
		input.Status.Route = url
		fUpdateCR = true
	}

	if fUpdateCR {
		// Now let's update the CR with the status data.
		err = r.Client.Status().Update(context.Background(), input)
		if err != nil {
			return false, err
		}
		return fUpdateCR, nil
	}
	return fUpdateCR, nil
}

//nolint:dupl
func (r *GiteaServerReconciler) finalizeObject(instance *gitopsv1alpha1.GiteaServer) error {
	// Add finalizer when object is created
	log.Printf("Finalizing GiteaServer object")

	// The object is being deleted
	if controllerutil.ContainsFinalizer(instance, gitopsv1alpha1.GiteaServerFinalizer) || controllerutil.ContainsFinalizer(instance, metav1.FinalizerOrphanDependents) {
		// Let's uninstall the Gitea Chart first
		if fUninstalled, err := UnInstallChart(instance.Spec.ReleaseName, instance.Spec.Namespace); !fUninstalled {
			log.Println("Chart [", instance.Spec.ReleaseName, "] could not uninstalled")
			log.Println("Error: ", err)
			return err
		}
		// List of PVCs
		pvcInfo := corev1.PersistentVolumeClaimList{
			TypeMeta: metav1.TypeMeta{},
			ListMeta: metav1.ListMeta{},
			Items:    []corev1.PersistentVolumeClaim{},
		}
		// We want the list from gitea namespace
		options := client.ListOptions{
			Namespace: Gitea_Namespace,
		}

		// List the pvcs
		// oc get pvc -n gitea
		if err := r.Client.List(context.Background(), &pvcInfo, &options); err == nil {
			if pvcInfo.Items != nil {
				deleteOptions := client.DeleteOptions{}
				for i := range pvcInfo.Items {
					err = r.Client.Delete(context.Background(), &pvcInfo.Items[i], &deleteOptions)
					if err != nil {
						log.Println("Could not delete pvc [", pvcInfo.Items[i].Name, "]")
						return err
					}
					log.Println("PVC [", pvcInfo.Items[i].Name, "] deleted successfully!")
				}
			}
		}
		// Finally we delete the gitea namespace
		if fDeleted, err := deleteNamespace(r.Client, Gitea_Namespace); !fDeleted && err != nil {
			log.Println("Namespace [", Gitea_Namespace, "] could not be deleted!")
			log.Println("Error: ", err)
			return err
		}
		log.Println("Namespace [", Gitea_Namespace, "] has been deleted!")
	}
	return nil
}
