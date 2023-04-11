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

	"github.com/Masterminds/semver/v3"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"

	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	//	olmapi "github.com/operator-framework/api/pkg/operators/v1alpha1"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

// PatternReconciler reconciles a Pattern object
type PatternReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger

	config         *rest.Config
	configClient   configclient.Interface
	argoClient     argoclient.Interface
	olmClient      olmclient.Interface
	fullClient     kubernetes.Interface
	dynamicClient  dynamic.Interface
	operatorClient operatorclient.OperatorV1Interface
	driftWatcher   driftWatcher
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=ingresses,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=list;get
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;get
//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list
//+kubebuilder:rbac:groups="operator.open-cluster-management.io",resources=multiclusterhubs,verbs=get;list
//+kubebuilder:rbac:groups=operator.openshift.io,resources="openshiftcontrollermanagers",resources=openshiftcontrollermanagers,verbs=get;list
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

	// Remove the ArgoCD application on deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add finalizer when object is created
		if !controllerutil.ContainsFinalizer(instance, api.PatternFinalizer) {
			controllerutil.AddFinalizer(instance, api.PatternFinalizer)
			err := r.Client.Update(context.TODO(), instance)
			return r.actionPerformed(instance, "updated finalizer", err)
		}

	} else if err := r.finalizeObject(instance); err != nil {
		return r.actionPerformed(instance, "finalize", err)

	} else {
		log.Printf("Removing finalizer from %s\n", instance.ObjectMeta.Name)
		controllerutil.RemoveFinalizer(instance, api.PatternFinalizer)
		if err = r.Client.Update(context.TODO(), instance); err != nil {
			log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", "remove finalizer", err.Error())
			return reconcile.Result{}, err
		}
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "finalize")
		return reconcile.Result{}, nil
	}

	// -- Fill in defaults (changes made to a copy and not persisted)
	err, qualifiedInstance := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	if qualifiedInstance.Spec.DynamicSubscriptions {
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
	}

	if err := r.preValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "prerequisite validation", err)
	}

	// -- Git Drift monitoring
	gitConfig := qualifiedInstance.Spec.GitConfig
	// if both git repositories are defined in the pattern's git configuration and the polling interval is not set to disable watching
	if gitConfig.OriginRepo != "" && gitConfig.TargetRepo != "" && gitConfig.PollInterval != -1 {
		if !r.driftWatcher.isWatching(qualifiedInstance.Name, qualifiedInstance.Namespace) {
			// start monitoring drifts for this pattern
			err := r.driftWatcher.add(qualifiedInstance.Name,
				qualifiedInstance.Namespace,
				gitConfig.PollInterval)
			if err != nil {
				return r.actionPerformed(qualifiedInstance, "add pattern to git drift watcher", err)
			}
		} else {
			err := r.driftWatcher.updateInterval(qualifiedInstance.Name, qualifiedInstance.Namespace, gitConfig.PollInterval)
			if err != nil {
				return r.actionPerformed(qualifiedInstance, "update the watch interval to git drift watcher", err)
			}
		}
	} else if r.driftWatcher.isWatching(qualifiedInstance.Name, qualifiedInstance.Namespace) {
		// The pattern has been updated an it no longer fulfills the conditions to monitor the drift
		err := r.driftWatcher.remove(qualifiedInstance.Name, qualifiedInstance.Namespace)
		if err != nil {
			return r.actionPerformed(qualifiedInstance, "remove pattern from git drift watcher", err)
		}
	}

	// -- GitOps Subscription
	targetSub := newSubscription(*qualifiedInstance)
	_ = controllerutil.SetOwnerReference(qualifiedInstance, targetSub, r.Scheme)

	_, sub := getSubscription(r.olmClient, targetSub.Name, targetSub.Namespace)
	if sub == nil {
		err := createSubscription(r.olmClient, targetSub)
		return r.actionPerformed(qualifiedInstance, "create gitops subscription", err)
	} else if ownedBySame(targetSub, sub) {
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

	// -- ArgoCD Application
	targetApp := newApplication(*qualifiedInstance, r.olmClient)
	_ = controllerutil.SetOwnerReference(qualifiedInstance, targetApp, r.Scheme)

	//log.Printf("Targeting: %s\n", objectYaml(targetApp))

	err, app := getApplication(r.argoClient, applicationName(*qualifiedInstance))
	if app == nil {
		log.Printf("App not found: %s\n", err.Error())
		err := createApplication(r.argoClient, targetApp)
		return r.actionPerformed(qualifiedInstance, "create application", err)

	} else if ownedBySame(targetApp, app) {
		// Check values
		err, changed := updateApplication(r.argoClient, targetApp, app)
		if changed {
			if err != nil {
				qualifiedInstance.Status.Version = 1 + qualifiedInstance.Status.Version
			}
			return r.actionPerformed(qualifiedInstance, "updated application", err)
		}

	} else {
		// Someone manually removed the owner ref
		return r.actionPerformed(qualifiedInstance, "create application", fmt.Errorf("We no longer own Application %q", targetApp.Name))
	}

	// Perform validation of the site values file(s)
	if err := r.postValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "validation", err)
	}
	// Report statistics

	log.Printf("\x1b[32;1m\tReconcile complete\x1b[0m\n")

	return ctrl.Result{}, nil
}

func validGitRepoURL(repoURL string) error {
	switch {
	case strings.HasPrefix(repoURL, "git@"):
		return errors.New(fmt.Errorf("invalid repository URL: %s", repoURL))
	case strings.HasPrefix(repoURL, "https://"),
		strings.HasPrefix(repoURL, "http://"):
		return nil
	default:
		return errors.New(fmt.Errorf("repository URL must be either http/https: %s", repoURL))
	}
}
func (r *PatternReconciler) preValidation(input *api.Pattern) error {
	// TARGET_REPO=$(shell git remote show origin | grep Push | sed -e 's/.*URL:[[:space:]]*//' -e 's%:[a-z].*@%@%' -e 's%:%/%' -e 's%git@%https://%' )
	gc := input.Spec.GitConfig
	if gc.OriginRepo != "" {
		if err := validGitRepoURL(gc.OriginRepo); err != nil {
			return err
		}
	}
	if gc.TargetRepo != "" {
		return validGitRepoURL(gc.TargetRepo)
	}
	return fmt.Errorf("TargetRepo cannot be empty")

	// Check the url is reachable

}

func (r *PatternReconciler) postValidation(input *api.Pattern) error {
	return nil
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (error, *api.Pattern) {

	output := input.DeepCopy()

	// Cluster ID:
	// oc get clusterversion -o jsonpath='{.items[].spec.clusterID}{"\n"}'
	// oc get clusterversion/version -o jsonpath='{.spec.clusterID}'
	if cv, err := r.configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{}); err != nil {
		return err, output
	} else {
		output.Status.ClusterID = string(cv.Spec.ClusterID)
	}

	// Cluster platform
	// oc get Infrastructure.config.openshift.io/cluster  -o jsonpath='{.spec.platformSpec.type}'
	clusterInfra, err := r.configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err, output
	} else {
		//   status:
		//    apiServerInternalURI: https://api-int.beekhof49.blueprints.rhecoeng.com:6443
		//    apiServerURL: https://api.beekhof49.blueprints.rhecoeng.com:6443
		//    controlPlaneTopology: HighlyAvailable
		//    etcdDiscoveryDomain: ""
		//    infrastructureName: beekhof49-pqzfb
		//    infrastructureTopology: HighlyAvailable
		//    platform: AWS
		//    platformStatus:
		//      aws:
		//        region: ap-southeast-2
		//      type: AWS

		output.Status.ClusterPlatform = string(clusterInfra.Spec.PlatformSpec.Type)
	}

	// Cluster Version
	// oc get OpenShiftControllerManager/cluster -o jsonpath='{.status.version}'
	clusterVersion, err := r.operatorClient.OpenShiftControllerManagers().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err, output
	} else {
		// status:
		//   ...
		//   version: 4.10.32
		v, version_err := semver.NewVersion(string(clusterVersion.Status.Version))
		if version_err != nil {
			return version_err, output
		}

		output.Status.ClusterVersion = fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	}

	// Derive cluster and domain names
	// oc get Ingress.config.openshift.io/cluster -o jsonpath='{.spec.domain}'
	clusterIngress, err := r.configClient.ConfigV1().Ingresses().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err, output
	}

	// "apps.mycluster.blueprints.rhecoeng.com"
	ss := strings.Split(clusterIngress.Spec.Domain, ".")

	output.Status.ClusterName = ss[1]
	output.Status.AppClusterDomain = clusterIngress.Spec.Domain
	output.Status.ClusterDomain = strings.Join(ss[1:], ".")

	if output.Spec.GitOpsConfig == nil {
		output.Spec.GitOpsConfig = &api.GitOpsConfig{}
	}

	if len(output.Spec.GitConfig.TargetRevision) == 0 {
		output.Spec.GitConfig.TargetRevision = "HEAD"
	}

	if len(output.Spec.GitConfig.OriginRevision) == 0 {
		output.Spec.GitConfig.OriginRevision = "HEAD"
	}

	if len(output.Spec.GitConfig.Hostname) == 0 {
		ss := strings.Split(output.Spec.GitConfig.TargetRepo, "/")
		output.Spec.GitConfig.Hostname = ss[2]
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

	// interval cannot be less than 180 seconds to avoid drowning the API server in requests
	// value of -1 effectivelly disables the watch for this pattern.
	if output.Spec.GitConfig.PollInterval > -1 && output.Spec.GitConfig.PollInterval < 180 {
		output.Spec.GitConfig.PollInterval = 180
	}

	if len(output.Status.Path) == 0 {
		output.Status.Path = filepath.Join(os.TempDir(), output.Namespace, output.Name)
	}

	return nil, output
}

func (r *PatternReconciler) authTokenFromSecret(namespace, secret, key string) (error, string) {
	if len(key) == 0 {
		return nil, ""
	}
	tokenSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: secret, Namespace: namespace}, tokenSecret)
	if err != nil {
		//      if tokenSecret, err = r.Client.Core().Secrets(namespace).Get(secret); err != nil {
		r.logger.Error(err, fmt.Sprintf("Could not obtain secret %s/%s", secret, namespace))
		return err, ""
	}

	if val, ok := tokenSecret.Data[key]; ok {
		// See also https://github.com/kubernetes/client-go/issues/198
		return nil, string(val)
	}
	return errors.New(fmt.Errorf("No key '%s' found in %s/%s", key, secret, namespace)), ""
}

func (r *PatternReconciler) finalizeObject(instance *api.Pattern) error {

	// Add finalizer when object is created
	log.Printf("Finalizing pattern object")

	// The object is being deleted
	if controllerutil.ContainsFinalizer(instance, api.PatternFinalizer) || controllerutil.ContainsFinalizer(instance, metav1.FinalizerOrphanDependents) {

		// Prepare the app for cascaded deletion
		err, qualifiedInstance := r.applyDefaults(instance)
		if err != nil {
			log.Printf("\n\x1b[31;1m\tCannot cleanup the ArgoCD application of an invalid pattern: %s\x1b[0m\n", err.Error())
			return nil
		}

		targetApp := newApplication(*qualifiedInstance, nil)
		_ = controllerutil.SetOwnerReference(qualifiedInstance, targetApp, r.Scheme)

		_, app := getApplication(r.argoClient, applicationName(*qualifiedInstance))
		if app == nil {
			log.Printf("Application has already been removed\n")
			return nil
		}

		if !ownedBySame(targetApp, app) {
			log.Printf("Application %q is not owned by us\n", app.Name)
			return nil
		}

		if r.driftWatcher.isWatching(qualifiedInstance.Name, qualifiedInstance.Namespace) {
			// Stop watching for drifts in the pattern's git repositories
			if err := r.driftWatcher.remove(instance.Name, instance.Namespace); err != nil {
				return err
			}
		}
		if _, changed := updateApplication(r.argoClient, targetApp, app); changed {
			return fmt.Errorf("updated application %q for removal\n", app.Name)
		}

		if haveACMHub(r) {
			return fmt.Errorf("waiting for removal of that acm hub")
		}

		if app.Status.Sync.Status == argoapi.SyncStatusCodeOutOfSync {
			return fmt.Errorf("application %q is still %s", app.Name, argoapi.SyncStatusCodeOutOfSync)
		}

		log.Printf("Removing the application, and cascading to anything instantiated by ArgoCD")
		if err := removeApplication(r.argoClient, app.Name); err != nil {
			return err
		}
		return fmt.Errorf("waiting for application %q to be removed\n", app.Name)
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

	if r.argoClient, err = argoclient.NewForConfig(r.config); err != nil {
		return err
	}

	if r.olmClient, err = olmclient.NewForConfig(r.config); err != nil {
		return err
	}

	if r.fullClient, err = kubernetes.NewForConfig(r.config); err != nil {
		return err
	}

	if r.dynamicClient, err = dynamic.NewForConfig(r.config); err != nil {
		return err
	}

	if r.operatorClient, err = operatorclient.NewForConfig(r.config); err != nil {
		return err
	}
	r.driftWatcher, _ = newDriftWatcher(r.Client, mgr.GetLogger(), newGitClient())
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Pattern{}).
		Complete(r)
}

func (r *PatternReconciler) onReconcileErrorWithRequeue(p *api.Pattern, reason string, err error, duration *time.Duration) (reconcile.Result, error) {
	// err is logged by the reconcileHandler
	p.Status.LastStep = reason
	if err != nil {
		p.Status.LastError = err.Error()
		log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", reason, err.Error())
		//r.logger.Error(fmt.Errorf("Reconcile step failed"), reason)

	} else {
		p.Status.LastError = ""
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", reason)
	}

	updateErr := r.Client.Status().Update(context.TODO(), p)
	if updateErr != nil {
		r.logger.Error(updateErr, "Failed to update Pattern status")
		return reconcile.Result{}, updateErr
	}
	if duration != nil {
		log.Printf("Requeueing\n")
		return reconcile.Result{RequeueAfter: *duration}, err
	}
	//	log.Printf("Reconciling with exponential duration")
	return reconcile.Result{}, err
}

func (r *PatternReconciler) actionPerformed(p *api.Pattern, reason string, err error) (reconcile.Result, error) {
	if err != nil {
		delay := time.Minute * 1
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	} else if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		delay := time.Minute * 2
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	}
	return r.onReconcileErrorWithRequeue(p, reason, err, nil)
}
