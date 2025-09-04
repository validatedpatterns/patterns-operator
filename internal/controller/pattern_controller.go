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
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"

	//	olmapi "github.com/operator-framework/api/pkg/operators/v1alpha1"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

const ReconcileLoopRequeueTime = 180 * time.Second

// PatternReconciler reconciles a Pattern object
type PatternReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	AnalyticsClient VpAnalyticsInterface

	logger logr.Logger

	config          *rest.Config
	configClient    configclient.Interface
	argoClient      argoclient.Interface
	olmClient       olmclient.Interface
	fullClient      kubernetes.Interface
	dynamicClient   dynamic.Interface
	routeClient     routeclient.Interface
	operatorClient  operatorclient.OperatorV1Interface
	driftWatcher    driftWatcher
	gitOperations   GitOperations
	giteaOperations GiteaOperations
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=ingresses,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=list;get
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch;delete;update;get;create;patch
//+kubebuilder:rbac:groups=argoproj.io,resources=argocds,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list
//+kubebuilder:rbac:groups="operator.open-cluster-management.io",resources=multiclusterhubs,verbs=get;list
//+kubebuilder:rbac:groups=operator.openshift.io,resources="openshiftcontrollermanagers",resources=openshiftcontrollermanagers,verbs=get;list
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;watch
//+kubebuilder:rbac:groups="route.openshift.io",namespace=vp-gitea,resources=routes;routes/custom-host,verbs=get;list;watch;create;update;patch;delete

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

	instance := &api.Pattern{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
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
	if instance.DeletionTimestamp.IsZero() {
		// Add finalizer when object is created
		if !controllerutil.ContainsFinalizer(instance, api.PatternFinalizer) {
			controllerutil.AddFinalizer(instance, api.PatternFinalizer)
			err = r.Update(context.TODO(), instance)
			return r.actionPerformed(instance, "updated finalizer", err)
		}
	} else if err = r.finalizeObject(instance); err != nil {
		return r.actionPerformed(instance, "finalize", err)
	} else {
		log.Printf("Removing finalizer from %s\n", instance.Name)
		controllerutil.RemoveFinalizer(instance, api.PatternFinalizer)
		if err = r.Update(context.TODO(), instance); err != nil {
			log.Printf("\x1b[31;1m\tReconcile step %q failed: %s\x1b[0m\n", "remove finalizer", err.Error())
			return reconcile.Result{}, err
		}
		log.Printf("\x1b[34;1m\tReconcile step %q complete\x1b[0m\n", "finalize")
		return reconcile.Result{}, nil
	}

	// -- Fill in defaults (changes made to a copy and not persisted)
	qualifiedInstance, err := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	if r.AnalyticsClient.SendPatternInstallationInfo(qualifiedInstance) {
		return r.actionPerformed(qualifiedInstance, "Updated status with identity sent", nil)
	}
	// Report loop completion statistics
	if r.AnalyticsClient.SendPatternStartEventInfo(qualifiedInstance) {
		return r.actionPerformed(qualifiedInstance, "Updated status with start event sent", nil)
	}

	gitConfig := qualifiedInstance.Spec.GitConfig
	// -- Git Drift monitoring
	// if both git repositories are defined in the pattern's git configuration and the polling interval is not set to disable watching
	if gitConfig.OriginRepo != "" && gitConfig.TargetRepo != "" && gitConfig.PollInterval != -1 {
		if !r.driftWatcher.isWatching(qualifiedInstance.Name, qualifiedInstance.Namespace) {
			// start monitoring drifts for this pattern
			err = r.driftWatcher.add(qualifiedInstance.Name,
				qualifiedInstance.Namespace,
				gitConfig.PollInterval)
			if err != nil {
				return r.actionPerformed(qualifiedInstance, "add pattern to git drift watcher", err)
			}
		} else {
			err = r.driftWatcher.updateInterval(qualifiedInstance.Name, qualifiedInstance.Namespace, gitConfig.PollInterval)
			if err != nil {
				return r.actionPerformed(qualifiedInstance, "update the watch interval to git drift watcher", err)
			}
		}
	} else if r.driftWatcher.isWatching(qualifiedInstance.Name, qualifiedInstance.Namespace) {
		// The pattern has been updated an it no longer fulfills the conditions to monitor the drift
		err = r.driftWatcher.remove(qualifiedInstance.Name, qualifiedInstance.Namespace)
		if err != nil {
			return r.actionPerformed(qualifiedInstance, "remove pattern from git drift watcher", err)
		}
	}

	// -- GitOps Subscription
	targetSub, _ := newSubscriptionFromConfigMap(r.fullClient)
	_ = controllerutil.SetOwnerReference(qualifiedInstance, targetSub, r.Scheme)

	sub, _ := getSubscription(r.olmClient, targetSub.Name)
	if sub == nil {
		err = createSubscription(r.olmClient, targetSub)
		return r.actionPerformed(qualifiedInstance, "create gitops subscription", err)
	} else if ownedBySame(targetSub, sub) {
		// Check version/channel etc
		// Dangerous if multiple patterns do not agree, or automatic upgrades are in place...
		changed, errSub := updateSubscription(r.olmClient, targetSub, sub)
		if changed {
			return r.actionPerformed(qualifiedInstance, "update gitops subscription", errSub)
		}
	} else {
		logOnce("The gitops subscription is not owned by us, leaving untouched")
	}
	logOnce("subscription found")

	clusterWideNS := getClusterWideArgoNamespace()
	if !haveNamespace(r.Client, clusterWideNS) {
		return r.actionPerformed(qualifiedInstance, "check application namespace", fmt.Errorf("waiting for creation"))
	}
	// Once we add support for creating the clusterwide argo in a separate NS we will uncomment this
	// else if !haveNamespace(r.Client, clusterWideNS) && *qualifiedInstance.Spec.Experimental { // create the namespace if it does not exist
	// 	 err = createNamespace(r.fullClient, clusterWideNS)
	//	 return r.actionPerformed(qualifiedInstance, "created vp clusterwide namespace", err)
	// }
	logOnce("namespace found")

	// Create the trusted-bundle configmap inside the clusterwide namespace
	errCABundle := createTrustedBundleCM(r.fullClient, getClusterWideArgoNamespace())
	if errCABundle != nil {
		return r.actionPerformed(qualifiedInstance, "error while creating trustedbundle cm", errCABundle)
	}

	// We only update the clusterwide argo instance so we can define our own 'initcontainers' section
	err = createOrUpdateArgoCD(r.dynamicClient, r.fullClient, ClusterWideArgoName, clusterWideNS)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "created or updated clusterwide argo instance", err)
	}
	// Copy the bootstrap secret to the namespaced argo namespace
	if qualifiedInstance.Spec.GitConfig.TokenSecret != "" {
		if err = r.copyAuthGitSecret(qualifiedInstance.Spec.GitConfig.TokenSecretNamespace,
			qualifiedInstance.Spec.GitConfig.TokenSecret, getClusterWideArgoNamespace(), "vp-private-repo-credentials"); err != nil {
			return r.actionPerformed(qualifiedInstance, "copying clusterwide git auth secret to namespaced argo", err)
		}
	}

	// If you specified OriginRepo then we automatically spawn a gitea instance via a special argo gitea application
	if gitConfig.OriginRepo != "" {
		giteaErr := r.createGiteaInstance(qualifiedInstance)
		if giteaErr != nil {
			return r.actionPerformed(qualifiedInstance, "error created gitea instance", giteaErr)
		}
	}

	ret, err := r.getLocalGit(qualifiedInstance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, ret, err)
	}

	targetApp := newArgoApplication(qualifiedInstance)
	_ = controllerutil.SetOwnerReference(qualifiedInstance, targetApp, r.Scheme)
	app, err := getApplication(r.argoClient, applicationName(qualifiedInstance), clusterWideNS)
	if app == nil {
		log.Printf("App not found: %s\n", err.Error())
		err = createApplication(r.argoClient, targetApp, clusterWideNS)
		return r.actionPerformed(qualifiedInstance, "create application", err)
	} else if ownedBySame(targetApp, app) {
		// Check values
		changed, errApp := updateApplication(r.argoClient, targetApp, app, clusterWideNS)
		if changed {
			if errApp != nil {
				qualifiedInstance.Status.Version = 1 + qualifiedInstance.Status.Version
			}
			_ = DropLocalGitPaths()

			return r.actionPerformed(qualifiedInstance, "updated application", errApp)
		}
	} else {
		// Someone manually removed the owner ref
		return r.actionPerformed(qualifiedInstance, "create application", fmt.Errorf("we no longer own Application %q", targetApp.Name))
	}

	// Copy the bootstrap secret to the namespaced argo namespace
	if qualifiedInstance.Spec.GitConfig.TokenSecret != "" {
		if err = r.copyAuthGitSecret(qualifiedInstance.Spec.GitConfig.TokenSecretNamespace,
			qualifiedInstance.Spec.GitConfig.TokenSecret, applicationName(qualifiedInstance), "vp-private-repo-credentials"); err != nil {
			return r.actionPerformed(qualifiedInstance, "copying clusterwide git auth secret to namespaced argo", err)
		}
	}
	// Perform validation of the site values file(s)
	if err = r.postValidation(qualifiedInstance); err != nil {
		return r.actionPerformed(qualifiedInstance, "validation", err)
	}

	// Update CR if necessary
	var fUpdate bool
	fUpdate, err = r.updatePatternCRDetails(qualifiedInstance)

	if err == nil && fUpdate {
		r.logger.Info("Pattern CR Updated")
	}

	// Report loop completion statistics
	if r.AnalyticsClient.SendPatternEndEventInfo(qualifiedInstance) {
		return r.actionPerformed(qualifiedInstance, "Updated status with end event sent", nil)
	}
	log.Printf("\x1b[32;1m\tReconcile complete\x1b[0m\n")

	result := ctrl.Result{
		Requeue:      false,
		RequeueAfter: ReconcileLoopRequeueTime,
	}

	return result, nil
}

func (r *PatternReconciler) createGiteaInstance(input *api.Pattern) error {
	gitConfig := input.Spec.GitConfig
	clusterWideNS := getClusterWideArgoNamespace()
	// The reason we create the vp-gitea namespace and and the
	// gitea-admin-secret is because otherwise it takes and extremely long time
	// to reconcile everything because the reconcile loop will be waiting a long time
	// for the namespace to show up and then the pod will take quite a while to retry
	// with the gitea-admin-secret mounted into it
	if !haveNamespace(r.Client, GiteaNamespace) {
		err := createNamespace(r.fullClient, GiteaNamespace)
		if err != nil {
			return fmt.Errorf("error creating %s namespace: %v", GiteaNamespace, err)
		}
	}
	var giteaAdminPassword string
	giteaAdminPassword, err := GenerateRandomPassword(GiteaDefaultPasswordLen, DefaultRandRead)
	if err != nil {
		return fmt.Errorf("error Generating gitea_admin password: %v", err)
	}

	secretData := map[string][]byte{
		"username": []byte(GiteaAdminUser),
		"password": []byte(giteaAdminPassword),
	}
	giteaAdminSecret := newSecret(GiteaAdminSecretName, GiteaNamespace, secretData, nil)
	err = r.Create(context.Background(), giteaAdminSecret)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create Gitea Admin Secret: %v", err)
	}

	log.Printf("Origin repo is set, creating gitea instance: %s", gitConfig.OriginRepo)
	giteaApp := newArgoGiteaApplication(input)
	_ = controllerutil.SetOwnerReference(input, giteaApp, r.Scheme)
	app, err := getApplication(r.argoClient, GiteaApplicationName, clusterWideNS)
	if app == nil {
		log.Printf("Gitea app not found: %s\n", err.Error())
		err = createApplication(r.argoClient, giteaApp, clusterWideNS)
		return fmt.Errorf("create gitea application: %v", err)
	} else if ownedBySame(giteaApp, app) {
		// Check values
		changed, errApp := updateApplication(r.argoClient, giteaApp, app, clusterWideNS)
		if changed {
			if errApp != nil {
				input.Status.Version = 1 + input.Status.Version
			}
			_ = DropLocalGitPaths()

			return fmt.Errorf("updated gitea application: %v", errApp)
		}
	} else {
		// Someone manually removed the owner ref
		return fmt.Errorf("we no longer own Application %q", giteaApp.Name)
	}
	if !haveNamespace(r.Client, GiteaNamespace) {
		return fmt.Errorf("waiting for giteanamespace creation")
	}

	// Here we need to call the gitea migration bits
	// Let's get the GiteaServer route
	giteaRouteURL, routeErr := getRoute(r.routeClient, GiteaRouteName, GiteaNamespace)
	if routeErr != nil {
		return fmt.Errorf("GiteaServer route not ready: %v", routeErr)
	}
	// Extract the repository name from the original target repo
	upstreamRepoName, repoErr := extractRepositoryName(gitConfig.OriginRepo)
	if repoErr != nil {
		return fmt.Errorf("error getting target Repo URL: %v", repoErr)
	}

	giteaRepoURL := fmt.Sprintf("%s/%s/%s", giteaRouteURL, GiteaAdminUser, upstreamRepoName)
	secret, secretErr := getSecret(r.fullClient, GiteaAdminSecretName, GiteaNamespace)
	if secretErr != nil {
		return fmt.Errorf("error getting gitea Admin Secret: %v", secretErr)
	}

	// Let's attempt to migrate the repo to Gitea
	_, _, err = r.giteaOperations.MigrateGiteaRepo(r.fullClient, string(secret.Data["username"]),
		string(secret.Data["password"]),
		input.Spec.GitConfig.OriginRepo,
		giteaRouteURL)
	if err != nil {
		return fmt.Errorf("GiteaServer Migrate Repository Error: %v", err)
	}

	// Migrate Repo has been done.
	// Replace the Target Repo with new Gitea Repo URL
	// and update the pattern CR
	input.Spec.GitConfig.TargetRepo = giteaRepoURL
	err = r.Update(context.Background(), input)
	if err != nil {
		return fmt.Errorf("update CR Target Repo: %v", err)
	}

	return nil
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

func (r *PatternReconciler) postValidation(input *api.Pattern) error { //nolint:revive
	return nil
}

func (r *PatternReconciler) applyDefaults(input *api.Pattern) (*api.Pattern, error) {
	output := input.DeepCopy()

	// Cluster ID:
	// oc get clusterversion -o jsonpath='{.items[].spec.clusterID}{"\n"}'
	// oc get clusterversion/version -o jsonpath='{.spec.clusterID}'
	if cv, err := r.configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{}); err != nil {
		return output, err
	} else {
		output.Status.ClusterID = string(cv.Spec.ClusterID)
	}

	// Cluster platform
	// oc get Infrastructure.config.openshift.io/cluster  -o jsonpath='{.spec.platformSpec.type}'
	clusterInfra, err := r.configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return output, err
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
	// oc get clusterversion/version -o yaml
	clusterVersions, err := r.configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return output, err
	} else {
		v, version_err := getCurrentClusterVersion(clusterVersions)
		if version_err != nil {
			return output, version_err
		}
		output.Status.ClusterVersion = fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	}

	// Derive cluster and domain names
	// oc get Ingress.config.openshift.io/cluster -o jsonpath='{.spec.domain}'
	clusterIngress, err := r.configClient.ConfigV1().Ingresses().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return output, err
	}

	// "apps.mycluster.blueprints.rhecoeng.com"
	ss := strings.Split(clusterIngress.Spec.Domain, ".")

	output.Status.ClusterName = ss[1]
	output.Status.AppClusterDomain = clusterIngress.Spec.Domain
	output.Status.ClusterDomain = strings.Join(ss[1:], ".")

	if output.Spec.GitOpsConfig == nil {
		output.Spec.GitOpsConfig = &api.GitOpsConfig{}
	}

	if output.Spec.GitConfig.TargetRevision == "" {
		output.Spec.GitConfig.TargetRevision = GitHEAD
	}

	if output.Spec.GitConfig.OriginRevision == "" {
		output.Spec.GitConfig.OriginRevision = GitHEAD
	}

	if output.Spec.GitConfig.Hostname == "" {
		hostname, err := extractGitFQDNHostname(output.Spec.GitConfig.TargetRepo)
		if err != nil {
			hostname = ""
		}
		output.Spec.GitConfig.Hostname = hostname
	}

	if output.Spec.MultiSourceConfig.Enabled == nil {
		multiSourceBool := true
		output.Spec.MultiSourceConfig.Enabled = &multiSourceBool
	}
	if output.Spec.ClusterGroupName == "" {
		output.Spec.ClusterGroupName = "default" //nolint:goconst
	}
	if output.Spec.MultiSourceConfig.HelmRepoUrl == "" {
		output.Spec.MultiSourceConfig.HelmRepoUrl = "https://charts.validatedpatterns.io/"
	}

	// interval cannot be less than 180 seconds to avoid drowning the API server in requests
	// value of -1 effectively disables the watch for this pattern.
	if output.Spec.GitConfig.PollInterval > -1 && output.Spec.GitConfig.PollInterval < 180 {
		output.Spec.GitConfig.PollInterval = 180
	}

	localCheckoutPath := getLocalGitPath(output.Spec.GitConfig.TargetRepo)
	if localCheckoutPath != output.Status.LocalCheckoutPath {
		_ = DropLocalGitPaths()
	}
	output.Status.LocalCheckoutPath = localCheckoutPath

	return output, nil
}

func (r *PatternReconciler) finalizeObject(instance *api.Pattern) error {
	// Add finalizer when object is created
	log.Printf("Finalizing pattern object")

	// The object is being deleted
	if controllerutil.ContainsFinalizer(instance, api.PatternFinalizer) || controllerutil.ContainsFinalizer(instance, metav1.FinalizerOrphanDependents) {
		// Prepare the app for cascaded deletion
		qualifiedInstance, err := r.applyDefaults(instance)
		if err != nil {
			log.Printf("\n\x1b[31;1m\tCannot cleanup the ArgoCD application of an invalid pattern: %s\x1b[0m\n", err.Error())
			return nil
		}
		ns := getClusterWideArgoNamespace()

		targetApp := newArgoApplication(qualifiedInstance)
		_ = controllerutil.SetOwnerReference(qualifiedInstance, targetApp, r.Scheme)

		app, _ := getApplication(r.argoClient, applicationName(qualifiedInstance), ns)
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
		if changed, _ := updateApplication(r.argoClient, targetApp, app, ns); changed {
			return fmt.Errorf("updated application %q for removal", app.Name)
		}

		if haveACMHub(r) {
			return fmt.Errorf("waiting for removal of that acm hub")
		}

		if app.Status.Sync.Status == argoapi.SyncStatusCodeOutOfSync {
			return fmt.Errorf("application %q is still %s", app.Name, argoapi.SyncStatusCodeOutOfSync)
		}

		log.Printf("Removing the application, and cascading to anything instantiated by ArgoCD")
		if err := removeApplication(r.argoClient, app.Name, ns); err != nil {
			return err
		}
		return fmt.Errorf("waiting for application %q to be removed", app.Name)
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

	if r.routeClient, err = routeclient.NewForConfig(r.config); err != nil {
		return err
	}
	r.driftWatcher, _ = newDriftWatcher(r.Client, mgr.GetLogger(), newGitClient())
	r.gitOperations = &GitOperationsImpl{}
	r.giteaOperations = &GiteaOperationsImpl{}

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
	return reconcile.Result{}, err
}

func (r *PatternReconciler) actionPerformed(p *api.Pattern, reason string, err error) (reconcile.Result, error) {
	if err != nil {
		delay := time.Minute * 1
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	} else if !p.DeletionTimestamp.IsZero() {
		delay := time.Minute * 2
		return r.onReconcileErrorWithRequeue(p, reason, err, &delay)
	}
	return r.onReconcileErrorWithRequeue(p, reason, err, nil)
}

// updatePatternCRDetails compares the current CR Status.Applications array
// against the instance.Status.Applications array.
// Returns true if the CR was updated else it returns false
func (r *PatternReconciler) updatePatternCRDetails(input *api.Pattern) (bool, error) {
	fUpdateCR := false
	var labelFilter = "validatedpatterns.io/pattern=" + input.Name

	// Copy just the applications
	// Used to compare against input which will be updated with
	// current application status
	existingApplications := input.Status.DeepCopy().Applications

	// Retrieving all applications that contain the label
	// oc get Applications -A -l validatedpatterns.io/pattern=<pattern-name>
	//
	// The VP framework adds the label to each application it creates.
	applications, err := r.argoClient.ArgoprojV1alpha1().Applications("").List(context.Background(),
		metav1.ListOptions{
			LabelSelector: labelFilter,
		})
	if err != nil {
		return false, err
	}

	// Reset the array
	input.Status.Applications = nil

	// Loop through the Pattern Applications and append the details to the Applications array
	// into input
	for _, app := range applications.Items { //nolint:gocritic // rangeValCopy: each iteration copies 936 bytes
		// Add Application information to ApplicationInfo struct
		var applicationInfo = api.PatternApplicationInfo{
			Name:             app.Name,
			Namespace:        app.Namespace,
			AppHealthStatus:  string(app.Status.Health.Status),
			AppHealthMessage: app.Status.Health.Message,
			AppSyncStatus:    string(app.Status.Sync.Status),
		}

		// Now let's append the Application Information
		input.Status.Applications = append(input.Status.Applications, applicationInfo)
	}

	// Check to see if the Pattern CR has a list of Applications
	// If it doesn't and we have a list of Applications
	// Let's update the Pattern CR and set the update flag to true
	if len(existingApplications) != len(input.Status.Applications) {
		fUpdateCR = true
	} else {
		// Compare the array items in the CR for the applications
		// with the current instance array
		for _, value := range input.Status.Applications {
			for _, existingValue := range existingApplications {
				// Check if AppSyncStatus or AppHealthStatus have been updated
				if value.Name == existingValue.Name &&
					(value.AppSyncStatus != existingValue.AppSyncStatus ||
						value.AppHealthStatus != existingValue.AppHealthStatus) {
					fUpdateCR = true
					break // We found a difference break out
				}
			}
		}
	}

	// Update the Pattern CR if difference was found
	if fUpdateCR {
		input.Status.LastStep = `update pattern application status`
		// Now let's update the CR with the application status data.
		err := r.Client.Status().Update(context.Background(), input)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func (r *PatternReconciler) authGitFromSecret(namespace, secret string) (map[string][]byte, error) {
	tokenSecret, err := r.fullClient.CoreV1().Secrets(namespace).Get(context.TODO(), secret, metav1.GetOptions{})
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("Could not obtain secret %s/%s", namespace, secret))
		return nil, err
	}
	return tokenSecret.Data, nil
}

func (r *PatternReconciler) copyAuthGitSecret(secretNamespace, secretName, destNamespace, destSecretName string) error {
	sourceSecret, err := r.authGitFromSecret(secretNamespace, secretName)
	if err != nil {
		return err
	}
	newSecretCopy := newSecret(destSecretName, destNamespace, sourceSecret, map[string]string{"argocd.argoproj.io/secret-type": "repository"})
	_, err = r.fullClient.CoreV1().Secrets(destNamespace).Get(context.TODO(), destSecretName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Resource does not exist, create it
			_, err = r.fullClient.CoreV1().Secrets(destNamespace).Create(context.TODO(), newSecretCopy, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// The destination secret already exists so we upate it and return an error if they were different so the reconcile loop can restart
	updatedSecret, err := r.fullClient.CoreV1().Secrets(destNamespace).Update(context.TODO(), newSecretCopy, metav1.UpdateOptions{})
	if err == nil && !compareMaps(newSecretCopy.Data, updatedSecret.Data) {
		return fmt.Errorf("the secret at %s/%s has been updated", destNamespace, destSecretName)
	}
	return err
}

func (r *PatternReconciler) getLocalGit(p *api.Pattern) (string, error) {
	var gitAuthSecret map[string][]byte
	var err error
	fmt.Printf("getLocalGit: %s", p.Status.LocalCheckoutPath)
	if p.Spec.GitConfig.TokenSecret != "" {
		if gitAuthSecret, err = r.authGitFromSecret(p.Spec.GitConfig.TokenSecretNamespace, p.Spec.GitConfig.TokenSecret); err != nil {
			return "obtaining git auth info from secret", err
		}
	}
	// Here we dump all the CAs in kube-root-ca.crt and in openshift-config-managed/trusted-ca-bundle to a file
	// and then we call git config --global http.sslCAInfo /path/to/your/cacert.pem
	// This makes us trust our self-signed CAs or any custom CAs a customer might have. We try and ignore any errors here
	if err = writeConfigMapKeyToFile(r.fullClient, "openshift-config-managed", "kube-root-ca.crt", "ca.crt", GitCustomCAFile, false); err != nil {
		fmt.Printf("Error while writing kube-root-ca.crt configmap to file: %v", err)
	}
	if err = writeConfigMapKeyToFile(r.fullClient, "openshift-config-managed", "trusted-ca-bundle", "ca-bundle.crt", GitCustomCAFile, true); err != nil {
		fmt.Printf("Error while appending trusted-ca-bundle configmap to file: %v", err)
	}

	gitDir := filepath.Join(p.Status.LocalCheckoutPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		err = cloneRepo(r.fullClient, r.gitOperations, p.Spec.GitConfig.TargetRepo, p.Status.LocalCheckoutPath, gitAuthSecret)
		if err != nil {
			return "cloning pattern repo", err
		}
	} else { // If the cloned repository directory already existed we check if the URL changed
		localURL, err := getGitRemoteURL(gitDir, "origin")
		if err != nil {
			return "getting remote URL pattern repo", err
		}
		if localURL != p.Spec.GitConfig.TargetRepo {
			fmt.Printf("Locally cloned URL is different from what is in the Spec, blowing away the folder and recloning")
			err = os.RemoveAll(gitDir)
			if err != nil {
				return "failed to remove locally cloned folder", err
			}
			err = cloneRepo(r.fullClient, r.gitOperations, p.Spec.GitConfig.TargetRepo, p.Status.LocalCheckoutPath, gitAuthSecret)
			if err != nil {
				return "cloning pattern repo after removal", err
			}
		}
	}
	if err := checkoutRevision(r.fullClient, r.gitOperations, p.Spec.GitConfig.TargetRepo, p.Status.LocalCheckoutPath,
		p.Spec.GitConfig.TargetRevision, gitAuthSecret); err != nil {
		return "checkout target revision", err
	}

	if err := r.preValidation(p); err != nil {
		return "prerequisite validation", err
	}
	return "", nil
}

func DropLocalGitPaths() error {
	// If there is a completely new local folder, let's remove the old one
	// User changed the target repo
	err := os.RemoveAll(filepath.Join(os.TempDir(), VPTmpFolder))
	if err != nil {
		return err
	}
	return nil
}
