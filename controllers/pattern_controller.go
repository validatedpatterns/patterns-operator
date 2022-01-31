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
	"os"
	"time"

	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	"github.com/google/uuid"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

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

	err, done := r.handleFinalizer(instance)
	if done {
		return r.actionPerformed(instance, "updated finalizer", err)
	}

	// Fill in defaults - Make changes in the copy?
	err, qualifiedInstance := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	// Check for gitops subscription
	needGitops := true

	// Update/create the argo application

	chart := chartForPattern(*qualifiedInstance)
	if chart == nil && len(qualifiedInstance.Status.Path) == 0 {
		err := r.prepareForClone(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "preparing the way", err)
	}

	gitDir := filepath.Join(qualifiedInstance.Status.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {

		var token string
		if err, token = r.authTokenFromSecret(qualifiedInstance.Spec.GitConfig.TokenSecret, qualifiedInstance.Spec.GitConfig.TokenSecretKey); err != nil {
			return r.actionPerformed(qualifiedInstance, "obtaining git auth token", err)
		}

		err := cloneRepo(qualifiedInstance.Spec.GitConfig.TargetRepo, qualifiedInstance.Status.Path, token)
		return r.actionPerformed(qualifiedInstance, "cloning pattern repo", err)
	}

	if err := checkoutRevision(qualifiedInstance.Status.Path, qualifiedInstance.Spec.GitConfig.TargetRevision); err != nil {
		return r.actionPerformed(qualifiedInstance, "checkout target revision", err)
	}
	
	if chart == nil {
		err := r.deployPattern(qualifiedInstance, needGitops, false)
		return r.actionPerformed(qualifiedInstance, "deploying the pattern", err)
	}

	// Reconcile any changes

	// Force a consistent value for bootstrap, which doesn't matter
	m := chart.Parameters["main"].(map[string]interface{})
	o := m["options"].(map[string]interface{})
	o["bootstrap"] = false

	actual, _ := yaml.Marshal(chart.Parameters)
	calculated, _ := yaml.Marshal(inputsForPattern(*qualifiedInstance, false))

	if string(calculated) != string(actual) {
		r.logger.Info("Parameters changed", "calculated:", string(calculated), "active:", string(actual))
		err := r.deployPattern(qualifiedInstance, false, false)
		return r.actionPerformed(qualifiedInstance, "updating the pattern", err)
	}

	// Perform validation of the site values file(s)
	// Report statistics

	return r.actionPerformed(qualifiedInstance, "done", nil)
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (error, *api.Pattern) {
	return nil, input
}

func (r *PatternReconciler) prepareForClone(p *api.Pattern) error {
	unique := uuid.New().URN()
	p.Status.Path = filepath.Join(os.TempDir(), p.Namespace, p.Name, unique)

	return os.MkdirAll(p.Status.Path, os.ModePerm)
}

func (r *PatternReconciler) authTokenFromSecret(secret types.NamespacedName, key string) (error, string) {
	if len(key) == 0 {
		return nil, ""
	}
	tokenSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), secret, tokenSecret)
	if err != nil {
		//	if tokenSecret, err = r.Client.Core().Secrets(secret.Namespace).Get(secret.Name); err != nil {
		r.logger.Error(fmt.Errorf("Could not obtain secret"), secret.Name, secret.Namespace)
		return err, ""
	}

	if val, ok := tokenSecret.Data[key]; ok {
		// See also https://github.com/kubernetes/client-go/issues/198
		return nil, string(val)
	}
	return fmt.Errorf("No key '%s' found in %s/%s", key, secret.Name, secret.Namespace), ""
}

func inputsForPattern(p api.Pattern, needGitOps bool) map[string]interface{} {
	inputs := map[string]interface{}{
		"main": map[string]interface{}{
			"git": map[string]interface{}{
				"repoURL":            p.Spec.GitConfig.TargetRepo,
				"revision":           p.Spec.GitConfig.TargetRevision,
				"valuesDirectoryURL": p.Spec.GitConfig.ValuesDirectoryURL,
			},
			"options": map[string]interface{}{
				"syncPolicy":          p.Spec.GitOpsConfig.SyncPolicy,
				"installPlanApproval": p.Spec.GitOpsConfig.InstallPlanApproval,
				"useCSV":              p.Spec.GitOpsConfig.UseCSV,
				"bootstrap":           needGitOps,
			},
			"gitops": map[string]interface{}{
				"channel": p.Spec.GitOpsConfig.OperatorChannel,
				"source":  p.Spec.GitOpsConfig.OperatorSource,
				"csv":     p.Spec.GitOpsConfig.OperatorCSV,
			},
			"siteName": p.Spec.SiteName,

			"global": map[string]interface{}{
				"imageregistry": map[string]interface{}{
					"type": "quay",
				},
				"git": map[string]interface{}{
					"hostname": p.Spec.GitConfig.Hostname,
					// Account is the user or organization under which the pattern repo lives
					"account": p.Spec.GitConfig.Account,
				},
			},
		},
	}
	return inputs
}

func (r *PatternReconciler) deployPattern(p *api.Pattern, needGitOps bool, isUpdate bool) error {

	chart := HelmChart{
		Name:       p.Name,
		Namespace:  p.ObjectMeta.Namespace,
		Version:    0,
		Path:       p.Status.Path,
		Parameters: inputsForPattern(*p, needGitOps),
	}

	var err error
	var version = 0
	if isUpdate {
		err, version = updateChart(chart)
	} else {
		err, version = installChart(chart)
	}
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

func (r *PatternReconciler) handleFinalizer(instance *api.Pattern) (error, bool) {

	// Add finalizer when object is created
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !ContainsString(instance.ObjectMeta.Finalizers, api.PatternFinalizer) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, api.PatternFinalizer)
			err := r.Client.Update(context.TODO(), instance)
			return err, true
		}

	} else {
		r.logger.Info("Deletion timestamp not zero")

		// The object is being deleted
		if ContainsString(instance.ObjectMeta.Finalizers, api.PatternFinalizer) || ContainsString(instance.ObjectMeta.Finalizers, metav1.FinalizerOrphanDependents) {
			// Do any required cleanup here

			// Remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = RemoveString(instance.ObjectMeta.Finalizers, api.PatternFinalizer)
			err := r.Client.Update(context.Background(), instance)
			return err, true
		}
	}

	return nil, false
}

// ContainsString checks if the string array contains the given string.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString removes the given string from the string array if exists.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}
