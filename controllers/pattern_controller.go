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
	"strings"
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

	configv1 "github.com/openshift/api/config/v1"
	networkv1 "github.com/openshift/api/config/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"

	olmapi "github.com/operator-framework/api/pkg/operators/v1alpha1"
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
	r.logger.Info("Reconciling Pattern")

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

	if err, done := r.handleFinalizer(instance); done || err != nil {
		return r.actionPerformed(instance, "updated finalizer", err)
	}

	// Fill in defaults - Make changes in the copy?
	err, qualifiedInstance := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	// Set needGitops based on existance of the argo subscription
	var needSubscription = true
	var clusterSubscriptions olmapi.SubscriptionList
	//	if tokenSecret, err = r.Client.Core().Secrets(secret.Namespace).Get(secret.Name); err != nil {

	if err := r.Client.List(context.TODO(), &clusterSubscriptions, &client.ListOptions{}); err == nil {
		for _, sub := range clusterSubscriptions.Items {
			if sub.Spec.Package == "openshift-gitops-operator" && sub.Namespace == "openshift-operators" {
				needSubscription = false
			}
		}
	}

	// Update/create the argo application

	chart := chartForPattern(*qualifiedInstance)
	if chart == nil && len(qualifiedInstance.Status.Path) == 0 {
		err := r.prepareForClone(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "preparing the way", err)
	}

	var token = ""
	if len(qualifiedInstance.Spec.GitConfig.TokenSecret.Name) > 0 {
		if err, token = r.authTokenFromSecret(qualifiedInstance.Spec.GitConfig.TokenSecret, qualifiedInstance.Spec.GitConfig.TokenSecretKey); err != nil {
			return r.actionPerformed(qualifiedInstance, "obtaining git auth token", err)
		}
	}

	gitDir := filepath.Join(qualifiedInstance.Status.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		err := cloneRepo(qualifiedInstance.Spec.GitConfig.TargetRepo, qualifiedInstance.Status.Path, token)
		return r.actionPerformed(qualifiedInstance, "cloning pattern repo", err)
	}

	if err := checkoutRevision(qualifiedInstance.Status.Path, token, qualifiedInstance.Spec.GitConfig.TargetRevision); err != nil {
		return r.actionPerformed(qualifiedInstance, "checkout target revision", err)
	}

	if err := r.preValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "prerequisite validation", err)
	}

	if chart == nil {
		err := r.deployPattern(qualifiedInstance, needSubscription, false)
		return r.actionPerformed(qualifiedInstance, "deploying the pattern", err)
	}

	// Reconcile any changes
	var needSync = false

	err, hash := repoHash(qualifiedInstance.Status.Path)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "obtain git hash", err)
	}

	if needSync == false && qualifiedInstance.Status.Revision != hash {
		needSync = true

	} else {

		// Force a consistent value for bootstrap, which doesn't matter
		m := chart.Parameters["main"].(map[string]interface{})
		o := m["options"].(map[string]interface{})
		o["bootstrap"] = false

		actual, _ := yaml.Marshal(chart.Parameters)
		calculated, _ := yaml.Marshal(inputsForPattern(*qualifiedInstance, false))

		if string(calculated) != string(actual) {
			r.logger.Info("Parameters changed", "calculated:", string(calculated), "active:", string(actual))
			needSync = true
		}
	}

	if needSync {
		err := r.deployPattern(qualifiedInstance, false, false)
		return r.actionPerformed(qualifiedInstance, "updating the pattern", err)
	}

	// Perform validation of the site values file(s)
	if err := r.postValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "validation", err)
	}
	// Report statistics

	return r.actionPerformed(qualifiedInstance, "done", nil)
}

func (r *PatternReconciler) preValidation(input *api.Pattern) error {

	//ss := strings.Compare(input.Spec.GitConfig.TargetRepo, "git")
	// TARGET_REPO=$(shell git remote show origin | grep Push | sed -e 's/.*URL:[[:space:]]*//' -e 's%:[a-z].*@%@%' -e 's%:%/%' -e 's%git@%https://%' )
	if index := strings.Index(input.Spec.GitConfig.TargetRepo, "git@"); index == 0 {
		return fmt.Errorf("Invalid TargetRepo: %s", input.Spec.GitConfig.TargetRepo)
	}

	return nil
}

func (r *PatternReconciler) postValidation(input *api.Pattern) error {
	return nil
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (error, *api.Pattern) {

	output := input.DeepCopy()

	// Cluster ID:
	// oc get clusterversion -o jsonpath='{.items[].spec.clusterID}{"\n"}'
	// oc get clusterversion/version -o jsonpath='{.spec.clusterID}'
	clusterVersion := &configv1.ClusterVersion{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "versiom"}, clusterVersion); err != nil {
		return err, output
	}

	r.logger.Info("clusterID:", clusterVersion.Spec.ClusterID)

	// Derive cluster and domain names
	// oc get Ingress.config.openshift.io/cluster -o jsonpath='{.spec.domain}'

	clusterIngress := &networkv1.Ingress{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, clusterIngress); err != nil {
		return err, output
	}

	//appDomain := "apps.mycluster.blueprints.rhecoeng.com"
	appDomain := clusterIngress.Spec.Domain
	domainElements := strings.Split(appDomain, ".")

	clustername := domainElements[1]
	domainname := domainElements[2:]
	r.logger.Info("cluster:", clustername, domainname)

	//global:
	//  datacenter:
	//    domain: blueprints.rhecoeng.com
	//    clustername: beekhof-gitops

	if len(output.Spec.GitConfig.Hostname) == 0 {
		ss := strings.Split(output.Spec.GitConfig.TargetRepo, "/")
		output.Spec.GitConfig.Hostname = ss[1]
	}

	// Set output.Spec.GitConfig.ValuesDirectoryURL based on the TargetRepo
	if len(output.Spec.GitConfig.ValuesDirectoryURL) == 0 && output.Spec.GitConfig.Hostname == "github.com" {
		// https://github.com/hybrid-cloud-patterns/industrial-edge/raw/main/
		var hash = "HEAD"
		if len(output.Spec.GitConfig.TargetRevision) > 0 {
			hash = output.Spec.GitConfig.TargetRevision
		}
		ss := fmt.Sprintf("%s/raw/%s/", output.Spec.GitConfig.TargetRepo, hash)
		output.Spec.GitConfig.ValuesDirectoryURL = strings.ReplaceAll(ss, ".git", "")
	}

	if len(output.Spec.GitConfig.Hostname) == 0 {
		ss := strings.Split(output.Spec.GitConfig.TargetRepo, "/")
		output.Spec.GitConfig.Hostname = ss[1]
	}

	if len(output.Spec.GitOpsConfig.SyncPolicy) == 0 {
		output.Spec.GitOpsConfig.SyncPolicy = api.InstallAutomatic
	}

	if len(output.Spec.GitOpsConfig.InstallPlanApproval) == 0 {
		output.Spec.GitOpsConfig.InstallPlanApproval = api.InstallAutomatic
	}

	if len(output.Spec.GitOpsConfig.OperatorChannel) == 0 {
		output.Spec.GitOpsConfig.OperatorChannel = "stable"
	}

	if len(output.Spec.GitOpsConfig.OperatorSource) == 0 {
		output.Spec.GitOpsConfig.OperatorSource = "redhat-operators"
	}
	if len(output.Spec.GitOpsConfig.OperatorCSV) == 0 {
		output.Spec.GitOpsConfig.OperatorCSV = "v1.4.0"
	}
	if len(output.Spec.SiteName) == 0 {
		output.Spec.SiteName = "default"
	}

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

func inputsForPattern(p api.Pattern, needSubscription bool) map[string]interface{} {
	gitMap := map[string]interface{}{
		"repoURL":  p.Spec.GitConfig.TargetRepo,
		"revision": p.Spec.GitConfig.TargetRevision,
	}

	if len(p.Spec.GitConfig.ValuesDirectoryURL) > 0 {
		gitMap["valuesDirectoryURL"] = p.Spec.GitConfig.ValuesDirectoryURL
	}

	inputs := map[string]interface{}{
		"main": map[string]interface{}{
			"git": gitMap,
			"options": map[string]interface{}{
				"syncPolicy":          p.Spec.GitOpsConfig.SyncPolicy,
				"installPlanApproval": p.Spec.GitOpsConfig.InstallPlanApproval,
				"useCSV":              p.Spec.GitOpsConfig.UseCSV,
				"bootstrap":           needSubscription,
			},
			"gitops": map[string]interface{}{
				"channel": p.Spec.GitOpsConfig.OperatorChannel,
				"source":  p.Spec.GitOpsConfig.OperatorSource,
				"csv":     p.Spec.GitOpsConfig.OperatorCSV,
			},
			"siteName": p.Spec.SiteName,
		},

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
	}
	return inputs
}

func (r *PatternReconciler) deployPattern(p *api.Pattern, needSubscription bool, isUpdate bool) error {

	chart := HelmChart{
		Name:       p.Name,
		Namespace:  p.ObjectMeta.Namespace,
		Version:    0,
		Path:       p.Status.Path,
		Parameters: inputsForPattern(*p, needSubscription),
	}

	var err error
	err, hash := repoHash(p.Status.Path)
	if err != nil {
		return err
	}

	var version = 0
	if isUpdate {
		err, version = updateChart(chart)
	} else {
		err, version = installChart(chart)
	}
	if err != nil {
		return err
	}

	r.logger.Info("Deployed %s/%s: %d.", p.Name, p.ObjectMeta.Namespace, version)
	p.Status.Version = version
	p.Status.Revision = hash
	return nil
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
