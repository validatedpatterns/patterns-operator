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
	"strings"
	"time"

	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	olmapi "github.com/operator-framework/api/pkg/operators/v1alpha1"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

// PatternReconciler reconciles a Pattern object
type PatternReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger

	config       *rest.Config
	configClient configclient.Interface
	fullClient   kubernetes.Interface
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=ingresses,verbs=list;get
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces;secrets;configmaps,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=list;get;create;update;patch;delete
//

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
	// Reconcile() should perform at most one action in any invocation
	// in order to simplify testing.
	r.logger = klog.FromContext(ctx)
	r.logger.Info("Reconciling Pattern")

	// Logger includes name and namespace
	// Its also wants arguments in pairs, eg.
	// r.logger.Error(err, fmt.Sprintf("[%s/%s] %s", p.Name, p.ObjectMeta.Namespace, reason))
	// Or r.logger.Error(err, "message", "name", p.Name, "namespace", p.ObjectMeta.Namespace, "reason", reason))

	// Fetch the NodeMaintenance instance
	instance := &api.Pattern{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.logger.Info("Pattern not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.logger.Info("Error reading the request object, requeuing.")
		return reconcile.Result{}, err
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add finalizer when object is created
		if !ContainsString(instance.ObjectMeta.Finalizers, api.PatternFinalizer) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, api.PatternFinalizer)
			err := r.Client.Update(context.TODO(), instance)
			return r.actionPerformed(instance, "updated finalizer", err)
		}

	} else if err := r.finalizeObject(instance); err != nil {
		return reconcile.Result{}, err

	} else {
		return reconcile.Result{}, nil
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

	if len(qualifiedInstance.Status.Path) == 0 {
		err := r.prepareForClone(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "preparing the way", err)

	} else if _, err := os.Stat(qualifiedInstance.Status.Path); os.IsNotExist(err) {
		err := r.prepareForClone(qualifiedInstance)
		return r.actionPerformed(qualifiedInstance, "preparing the way", err)
	}

	var token = ""
	if len(qualifiedInstance.Spec.GitConfig.TokenSecret) > 0 {
		if err, token = r.authTokenFromSecret(qualifiedInstance.Spec.GitConfig.TokenSecretNamespace, qualifiedInstance.Spec.GitConfig.TokenSecret, qualifiedInstance.Spec.GitConfig.TokenSecretKey); err != nil {
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

	if isPatternDeployed(qualifiedInstance.Name) == false {
		// Helm doesn't always realize right away...
		// Can result in unnecessary second deployment
		err := r.deployPattern(qualifiedInstance, "deploying the pattern", needSubscription, false)
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
		var err error
		var deployedMarshalled []byte
		var calculatedMarshalled []byte
		// Force a consistent value for bootstrap, which doesn't matter
		if err, current := getChartValues(qualifiedInstance.Name); err == nil {
			m := current["main"].(map[string]interface{})
			o := m["options"].(map[string]interface{})
			o["bootstrap"] = false
			if deployedMarshalled, err = yaml.Marshal(current); err != nil {
				needSync = true
				r.logger.Info("Error marshalling deployed values", "input", current, "error", err.Error())
			}
		} else {
			needSync = true
			r.logger.Info("Error obtaining deployed values", "input", current, "error", err.Error())
		}
		//calculated, _ := yaml.Marshal(inputsForPattern(*qualifiedInstance, false))

		err, calculated := coalesceChartValues(*qualifiedInstance)
		if err != nil {
			needSync = true
			log.Printf("Error coalescing calculated values: %s", err.Error())

		} else if calculatedMarshalled, err = yaml.Marshal(calculated); err != nil {
			needSync = true
			r.logger.Info("Error marshalling calculated values", "input", calculated, "error", err.Error())
		}

		if string(calculatedMarshalled) != string(deployedMarshalled) {
			log.Printf(fmt.Sprintf("Parameters changed. calculated...\n%s\nactual...\n%s\n", string(calculatedMarshalled), string(deployedMarshalled)))
			needSync = true
		} else {
			log.Printf("Parameters unchanged\n")
		}
	}

	if needSync {
		err := r.deployPattern(qualifiedInstance, "updating the pattern", false, true)
		return r.actionPerformed(qualifiedInstance, "updating the pattern", err)
	}

	// Perform validation of the site values file(s)
	if err := r.postValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "validation", err)
	}
	// Report statistics

	minutes := time.Duration(qualifiedInstance.Spec.ReconcileMinutes)
	log.Printf("\n\x1b[32;1m\tReconcile complete - waiting %d minutes\x1b[0m\n", minutes)

	return ctrl.Result{RequeueAfter: time.Minute * minutes}, nil
}

func (r *PatternReconciler) preValidation(input *api.Pattern) error {

	//ss := strings.Compare(input.Spec.GitConfig.TargetRepo, "git")
	// TARGET_REPO=$(shell git remote show origin | grep Push | sed -e 's/.*URL:[[:space:]]*//' -e 's%:[a-z].*@%@%' -e 's%:%/%' -e 's%git@%https://%' )
	if index := strings.Index(input.Spec.GitConfig.TargetRepo, "git@"); index == 0 {
		return errors.New(fmt.Errorf("Invalid TargetRepo: %s", input.Spec.GitConfig.TargetRepo))
	}

	return nil
}

func (r *PatternReconciler) postValidation(input *api.Pattern) error {
	return nil
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (error, *api.Pattern) {

	output := input.DeepCopy()
	if output.Spec.ReconcileMinutes == 0 {
		output.Spec.ReconcileMinutes = 10
	}

	// Cluster ID:
	// oc get clusterversion -o jsonpath='{.items[].spec.clusterID}{"\n"}'
	// oc get clusterversion/version -o jsonpath='{.spec.clusterID}'
	if cv, err := r.configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{}); err != nil {
		return err, output
	} else {
		output.Status.ClusterID = string(cv.Spec.ClusterID)
	}

	// Derive cluster and domain names
	// oc get Ingress.config.openshift.io/cluster -o jsonpath='{.spec.domain}'
	clusterIngress, err := r.configClient.ConfigV1().Ingresses().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err, output
	}

	// "apps.mycluster.blueprints.rhecoeng.com"
	output.Status.ClusterDomain = clusterIngress.Spec.Domain

	if len(output.Spec.GitConfig.TargetRevision) == 0 {
		output.Spec.GitConfig.TargetRevision = "main"
	}

	// Set output.Spec.GitConfig.ValuesDirectoryURL based on the TargetRepo
	if len(output.Spec.GitConfig.ValuesDirectoryURL) == 0 && output.Spec.GitConfig.Hostname == "github.com" {
		// https://github.com/hybrid-cloud-patterns/industrial-edge/raw/main/
		ss := fmt.Sprintf("%s/raw/%s/", output.Spec.GitConfig.TargetRepo, output.Spec.GitConfig.TargetRevision)
		output.Spec.GitConfig.ValuesDirectoryURL = strings.ReplaceAll(ss, ".git", "")
	}

	if len(output.Spec.GitConfig.Hostname) == 0 {
		ss := strings.Split(output.Spec.GitConfig.TargetRepo, "/")
		output.Spec.GitConfig.Hostname = ss[2]
	}

	if output.Spec.GitOpsConfig == nil {
		output.Spec.GitOpsConfig = &api.GitOpsConfig{}
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
	if len(output.Spec.ClusterGroupName) == 0 {
		output.Spec.ClusterGroupName = "default"
	}

	return nil, output
}

func (r *PatternReconciler) prepareForClone(p *api.Pattern) error {
	p.Status.Path = filepath.Join(os.TempDir(), p.Namespace, p.Name)
	return os.MkdirAll(p.Status.Path, os.ModePerm)
}

func (r *PatternReconciler) authTokenFromSecret(namespace, secret, key string) (error, string) {
	if len(key) == 0 {
		return nil, ""
	}
	tokenSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: secret, Namespace: namespace}, tokenSecret)
	if err != nil {
		//	if tokenSecret, err = r.Client.Core().Secrets(namespace).Get(secret); err != nil {
		r.logger.Error(err, fmt.Sprintf("Could not obtain secret %s/%s", secret, namespace))
		return err, ""
	}

	if val, ok := tokenSecret.Data[key]; ok {
		// See also https://github.com/kubernetes/client-go/issues/198
		return nil, string(val)
	}
	return errors.New(fmt.Errorf("No key '%s' found in %s/%s", key, secret, namespace)), ""
}

func (r *PatternReconciler) deployPattern(p *api.Pattern, step string, needSubscription bool, isUpdate bool) error {

	var err error
	err, hash := repoHash(p.Status.Path)
	if err != nil {
		return err
	}

	log.Printf("debug overwrite")
	_, _ = overwriteWithChart(*p)

	var version = 0
	if p.Status.LastStep == step {
		log.Printf("Escalating %s", step)
		err, version = overwriteWithChart(*p)

	} else if isUpdate {
		log.Printf("updating pattern")
		err, version = updateChart(*p)
	} else {
		log.Printf("installing pattern")
		err, version = installChart(*p)
	}
	if err != nil {
		return err
	}

	log.Printf("Deployed %s/%s: %d.", p.Name, p.ObjectMeta.Namespace, version)

	if err, deployed := getChartValues(p.Name); err == nil {
		if deployedMarshalled, err := yaml.Marshal(deployed); err == nil {
			log.Printf("Deployed values:\n%s\n", string(deployedMarshalled))
		}
	}
	p.Status.Version = version
	p.Status.Revision = hash
	return nil
}

func (r *PatternReconciler) finalizeObject(instance *api.Pattern) error {

	// Add finalizer when object is created
	log.Printf("Finalizing pattern object")

	// The object is being deleted
	if ContainsString(instance.ObjectMeta.Finalizers, api.PatternFinalizer) || ContainsString(instance.ObjectMeta.Finalizers, metav1.FinalizerOrphanDependents) {
		// Do any required cleanup here
		log.Printf("Uninstalling")

		if err := uninstallChart(instance.Name); err != nil {
			// Best effort only...
			r.logger.Info("Could not uninstall pattern", "error", err)
		}

		// Remove our finalizer from the list and update it.
		instance.ObjectMeta.Finalizers = RemoveString(instance.ObjectMeta.Finalizers, api.PatternFinalizer)
		err := r.Client.Update(context.Background(), instance)
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PatternReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.config = mgr.GetConfig()

	if r.configClient, err = configclient.NewForConfig(r.config); err != nil {
		return err
	}

	if r.fullClient, err = kubernetes.NewForConfig(r.config); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Pattern{}).
		Complete(r)
}

func (r *PatternReconciler) onReconcileErrorWithRequeue(p *api.Pattern, reason string, err error, duration *time.Duration) (reconcile.Result, error) {
	// err is logged by the reconcileHandler
	p.Status.LastStep = reason
	if err != nil {
		p.Status.LastError = err.Error()
		log.Printf("\n\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", reason, err.Error())
		//r.logger.Error(fmt.Errorf("Reconcile step failed"), reason)
	} else {
		p.Status.LastError = ""
		log.Printf("\n\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", reason)
	}

	updateErr := r.Client.Status().Update(context.TODO(), p)
	if updateErr != nil {
		r.logger.Error(updateErr, "Failed to update Pattern status")
	}
	if duration != nil {
		return reconcile.Result{RequeueAfter: *duration}, err
	}
	//	log.Printf("Reconciling with exponential duration")
	return reconcile.Result{}, err
}

func (r *PatternReconciler) actionPerformed(p *api.Pattern, reason string, err error) (reconcile.Result, error) {
	if err == nil {
		delay := time.Second * 5
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	}
	return r.onReconcileErrorWithRequeue(p, reason, err, nil)
}
