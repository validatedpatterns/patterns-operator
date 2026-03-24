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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hybrid-cloud-patterns/patterns-operator/internal/controller/console"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoclient "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	v1 "github.com/operator-framework/api/pkg/operators/v1"

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
	gitOperations   GitOperations
	giteaOperations GiteaOperations

	mgr                ctrl.Manager
	ctrl               crcontroller.Controller
	argoCDWatchStarted bool
}

//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gitops.hybrid-cloud-patterns.io,resources=patterns/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=ingresses,verbs=list;get
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=list;get
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch;delete;update;get;create;patch
//+kubebuilder:rbac:groups=console.openshift.io,resources=consolelinks,verbs=get;list;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=argocds,verbs=list;watch;get;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=operatorgroups,verbs=list;get;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="operator.open-cluster-management.io",resources=multiclusterhubs,verbs=get;list
//+kubebuilder:rbac:groups=operator.openshift.io,resources="openshiftcontrollermanagers",resources=openshiftcontrollermanagers,verbs=get;list
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;watch
//+kubebuilder:rbac:groups="route.openshift.io",namespace=vp-gitea,resources=routes;routes/custom-host,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="view.open-cluster-management.io",resources=managedclusterviews,verbs=create
//+kubebuilder:rbac:groups="cluster.open-cluster-management.io",resources=managedclusters,verbs=list;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// The Reconcile function compares the state specified by
// the Pattern object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *PatternReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:funlen
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

	// Ensure console plugin is registered and enabled
	if err := console.CreateOrUpdatePlugin(ctx, r.Client); err != nil {
		r.logger.Error(err, "failed to create/update console plugin")
	}
	if err := console.EnablePlugin(ctx, r.Client); err != nil {
		r.logger.Error(err, "failed to enable console plugin")
	}

	// -- Fill in defaults (changes made to a copy and not persisted)
	qualifiedInstance, err := r.applyDefaults(instance)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "applying defaults", err)
	}

	// -- Detect ArgoCD namespace (legacy upgrade vs greenfield)
	// Must happen before subscription creation since it controls DISABLE_DEFAULT_ARGOCD_INSTANCE.
	detectArgoNamespace(r.dynamicClient)

	if r.AnalyticsClient.SendPatternInstallationInfo(qualifiedInstance) {
		return r.actionPerformed(qualifiedInstance, "Updated status with identity sent", nil)
	}
	// Report loop completion statistics
	if r.AnalyticsClient.SendPatternStartEventInfo(qualifiedInstance) {
		return r.actionPerformed(qualifiedInstance, "Updated status with start event sent", nil)
	}

	// -- GitOps Subscription
	if done, result, subErr := r.reconcileGitOpsSubscription(qualifiedInstance); done {
		return result, subErr
	}
	logOnce("subscription found")

	// Dynamically add an ArgoCD watch once the GitOps operator is installed
	// and the CRD is available. This is a no-op after the first successful call.
	r.startArgoCDWatch()

	clusterWideNS := getClusterWideArgoNamespace()
	if !haveNamespace(r.Client, clusterWideNS) {
		if isLegacyArgoNamespace() {
			// Legacy mode: wait for the gitops-operator to create the openshift-gitops namespace
			return r.actionPerformed(qualifiedInstance, "check application namespace", fmt.Errorf("waiting for creation"))
		}
		// Greenfield: create the namespace ourselves
		if err = createNamespace(r.fullClient, clusterWideNS); err != nil {
			return r.actionPerformed(qualifiedInstance, "error creating ArgoCD namespace", err)
		}
		return r.actionPerformed(qualifiedInstance, "created ArgoCD namespace", nil)
	}
	logOnce("namespace found")

	// Create the trusted-bundle configmap inside the clusterwide namespace
	errCABundle := createTrustedBundleCM(r.fullClient, getClusterWideArgoNamespace())
	if errCABundle != nil {
		return r.actionPerformed(qualifiedInstance, "error while creating trustedbundle cm", errCABundle)
	}

	// Wait for the trusted-ca-bundle configmap to be populated by the cluster network operator
	// before creating the ArgoCD CR. This prevents a race where the repo-server init container
	// runs before the CA bundle is injected, leaving ArgoCD unable to verify public TLS certs.
	populated, errPopulated := isTrustedBundleCMPopulated(r.fullClient, getClusterWideArgoNamespace())
	if errPopulated != nil {
		return r.actionPerformed(qualifiedInstance, "error checking trusted-ca-bundle population", errPopulated)
	}
	if !populated {
		return r.actionPerformed(qualifiedInstance, "waiting for trusted-ca-bundle to be populated",
			fmt.Errorf("trusted-ca-bundle configmap in %s not yet populated by cluster network operator", getClusterWideArgoNamespace()))
	}

	// We only update the clusterwide argo instance so we can define our own 'initcontainers' section
	err = createOrUpdateArgoCD(r.dynamicClient, r.fullClient, getClusterWideArgoName(), clusterWideNS)
	if err != nil {
		return r.actionPerformed(qualifiedInstance, "created or updated clusterwide argo instance", err)
	}

	// Create/update the ConsoleLink so the ArgoCD instance appears in the OpenShift console nine-box menu
	if !isLegacyArgoNamespace() && qualifiedInstance.Status.AppClusterDomain != "" {
		if err = createOrUpdateConsoleLink(r.dynamicClient, getClusterWideArgoName(), clusterWideNS, qualifiedInstance.Status.AppClusterDomain); err != nil {
			return r.actionPerformed(qualifiedInstance, "error creating ConsoleLink for ArgoCD", err)
		}
	}

	// Copy the bootstrap secret to the namespaced argo namespace
	if qualifiedInstance.Spec.GitConfig.TokenSecret != "" {
		if err = r.copyAuthGitSecret(qualifiedInstance.Spec.GitConfig.TokenSecretNamespace,
			qualifiedInstance.Spec.GitConfig.TokenSecret, getClusterWideArgoNamespace(), "vp-private-repo-credentials"); err != nil {
			return r.actionPerformed(qualifiedInstance, "copying clusterwide git auth secret to namespaced argo", err)
		}
	}

	// If you specified OriginRepo then we automatically spawn a gitea instance via a special argo gitea application
	if qualifiedInstance.Spec.GitConfig.OriginRepo != "" {
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

	// Report loop completion statistics (fire-and-forget, don't interrupt reconcile completion)
	r.AnalyticsClient.SendPatternEndEventInfo(qualifiedInstance)

	log.Printf("\x1b[32;1m\tReconcile complete\x1b[0m\n")

	if qualifiedInstance.Status.LastStep != "reconcile complete" || qualifiedInstance.Status.LastError != "" {
		qualifiedInstance.Status.LastStep = "reconcile complete"
		qualifiedInstance.Status.LastError = ""
		if updateErr := r.Client.Status().Update(context.TODO(), qualifiedInstance); updateErr != nil {
			r.logger.Error(updateErr, "Failed to update Pattern status")
		}
	}

	result := ctrl.Result{
		Requeue:      false,
		RequeueAfter: ReconcileLoopRequeueTime,
	}

	return result, nil
}

// reconcileGitOpsSubscription ensures the GitOps operator subscription exists and is up-to-date.
// It returns (done, result, err) — when done is true the caller should return result/err immediately.
func (r *PatternReconciler) reconcileGitOpsSubscription(qualifiedInstance *api.Pattern) (done bool, result ctrl.Result, err error) {
	// Only disable the default ArgoCD instance for non-legacy deployments.
	// For legacy deployments, the gitops-operator's default instance is still in use.
	disableDefault := !isLegacyArgoNamespace()
	targetSub, err := newSubscriptionFromConfigMap(r.fullClient, disableDefault)

	if err != nil {
		res, e := r.actionPerformed(qualifiedInstance, "error creating new subscription from configmap", err)
		return true, res, e
	}
	subscriptionName, subscriptionNamespace := DetectGitOpsSubscription()
	// If the pattern operator is installed to the new vp namespace we need to create a ns, operatorgroup for the new sub
	if DetectOperatorNamespace() != LegacyOperatorNamespace {
		// Create namespace for gitops subscription
		if err := createNamespace(r.fullClient, subscriptionNamespace); err != nil {
			res, e := r.actionPerformed(qualifiedInstance, "error creating namespace for gitops subscription", err)
			return true, res, e
		}

		// Create operatorgroup for gitops subscription
		var og *v1.OperatorGroup
		if og, err = getOperatorGroup(r.olmClient, subscriptionNamespace); err != nil {
			res, e := r.actionPerformed(qualifiedInstance, "error getting operatorgroup for gitops subscription", err)
			return true, res, e
		}
		if og == nil {
			if err := createOperatorGroup(r.olmClient, subscriptionNamespace); err != nil {
				res, e := r.actionPerformed(qualifiedInstance, "error creating operatorgroup for gitops subscription", err)
				return true, res, e
			}
		}
	}

	currentSub, err := getSubscription(r.olmClient, subscriptionName, subscriptionNamespace)
	if err != nil {
		res, e := r.actionPerformed(qualifiedInstance, "error getting gitops subscription", err)
		return true, res, e
	}

	if currentSub == nil {
		if err = createSubscription(r.olmClient, targetSub); err != nil {
			res, e := r.actionPerformed(qualifiedInstance, "error creating gitops subscription", err)
			return true, res, e
		}
	} else {
		// Remove any stale owner references from the subscription (historically set by
		// the pattern or the operator configmap). Cross-namespace owner references are
		// not allowed, so we clean them up and rely on the subscription persisting
		// independently.
		changed := false
		if err := controllerutil.RemoveOwnerReference(qualifiedInstance, currentSub, r.Scheme); err == nil {
			changed = true
		}
		operatorConfigMap, cmErr := GetOperatorConfigmap()
		if cmErr == nil {
			if err := controllerutil.RemoveOwnerReference(operatorConfigMap, currentSub, r.Scheme); err == nil {
				changed = true
			}
		}
		if changed {
			if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(currentSub.Namespace).Update(context.Background(), currentSub, metav1.UpdateOptions{}); err != nil {
				res, e := r.actionPerformed(qualifiedInstance, "error removing stale owner references from gitops subscription", err)
				return true, res, e
			}
			res, e := r.actionPerformed(qualifiedInstance, "removed stale owner references from gitops subscription", nil)
			return true, res, e
		}

		// Check version/channel etc
		updatedSub, errSub := updateSubscription(r.olmClient, targetSub, currentSub)
		if updatedSub {
			res, e := r.actionPerformed(qualifiedInstance, "update gitops subscription", errSub)
			return true, res, e
		}
	}

	return false, ctrl.Result{}, nil
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

	localCheckoutPath := getLocalGitPath(output.Spec.GitConfig.TargetRepo)
	if localCheckoutPath != output.Status.LocalCheckoutPath {
		_ = DropLocalGitPaths()
	}
	output.Status.LocalCheckoutPath = localCheckoutPath

	return output, nil
}

func (r *PatternReconciler) updateDeletionPhase(instance *api.Pattern, phase api.PatternDeletionPhase) error {
	log.Printf("Updating deletion phase to '%s'", phase)
	instance.Status.DeletionPhase = phase
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
		return fmt.Errorf("failed to update deletion phase: %w", err)
	}

	// Re-fetch to get updated status
	if err := r.Get(context.TODO(), client.ObjectKeyFromObject(instance), instance); err != nil {
		return fmt.Errorf("failed to re-fetch pattern after phase update: %w", err)
	}

	return nil
}

func (r *PatternReconciler) deleteSpokeApps(targetApp, app *argoapi.Application, namespace string) error {
	log.Printf("Deletion phase: %s - checking if all child applications are gone from spoke", api.DeleteSpokeChildApps)

	// Update application with deletePattern=DeleteSpokeChildApps to trigger spoke child deletion
	changed, errUpdate := updateApplication(r.argoClient, targetApp, app, namespace)
	if errUpdate != nil {
		return fmt.Errorf("failed to update application %q for spoke child deletion: %v", app.Name, errUpdate)
	}
	if changed {
		return fmt.Errorf("updated application %q for spoke child deletion", app.Name)
	}

	if err := syncApplication(r.argoClient, app, false); err != nil {
		return err
	}

	childApps, err := getChildApplications(r.argoClient, app)
	if err != nil {
		return err
	}

	for _, childApp := range childApps { //nolint:gocritic // rangeValCopy: each iteration copies 992 bytes
		if err := syncApplication(r.argoClient, &childApp, false); err != nil {
			return err
		}
	}

	// Check if all child applications are gone from spoke
	allGone, err := r.checkSpokeApplicationsGone(false)
	if err != nil {
		return fmt.Errorf("error checking child applications: %w", err)
	}

	if !allGone {
		return fmt.Errorf("waiting for child applications to be deleted from spoke clusters")
	}

	return nil
}

func (r *PatternReconciler) deleteHubApps(targetApp, app *argoapi.Application, namespace string) error {
	log.Printf("Deletion phase: %s - deleting child apps from hub", api.DeleteHubChildApps)

	childApps, err := getChildApplications(r.argoClient, app)
	if err != nil {
		return fmt.Errorf("failed to get child applications: %w", err)
	}

	if len(childApps) == 0 {
		return nil
	}
	// Delete managed clusters (excluding local-cluster)
	// These must be removed before hub deletion can proceed because ACM won't delete properly if they exist
	// we do not care about the error, since we might be on a standalone cluster
	managedClusters, _ := r.listManagedClusters(context.Background())

	if len(managedClusters) > 0 {
		deletedCount, err := r.deleteManagedClusters(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to delete managed clusters: %w", err)
		}

		if deletedCount > 0 {
			log.Printf("Deleted %d managed cluster(s), waiting for them to be fully removed", deletedCount)
			return fmt.Errorf("deleted %d managed cluster(s), waiting for removal to complete before proceeding with hub deletion", deletedCount)
		}
	}

	// Update application with deletePattern=DeleteHubChildApps to trigger hub child app deletion
	changed, errUpdate := updateApplication(r.argoClient, targetApp, app, namespace)
	if errUpdate != nil {
		return fmt.Errorf("failed to update application %q for hub deletion: %v", app.Name, errUpdate)
	}
	if changed {
		return fmt.Errorf("updated application %q for hub deletion", app.Name)
	}

	if err := syncApplication(r.argoClient, app, true); err != nil {
		return err
	}

	return fmt.Errorf("waiting %d hub child applications to be removed", len(childApps))
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
		// Ensure detection has run for the finalize path
		detectArgoNamespace(r.dynamicClient)
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

		// Initialize deletion phase if not set
		if qualifiedInstance.Status.DeletionPhase == api.InitializeDeletion {
			log.Printf("Initializing deletion phase")
			if haveACMHub(r) {
				if err := r.updateDeletionPhase(qualifiedInstance, api.DeleteSpokeChildApps); err != nil {
					return err
				}
			} else {
				// There is no acm/spoke, we can directly start cleaning up child apps (from hub)
				if err := r.updateDeletionPhase(qualifiedInstance, api.DeleteHubChildApps); err != nil {
					return err
				}
			}

			return fmt.Errorf("initialized deletion phase, requeueing now")
		}

		// Phase 1: Delete child applications from spoke clusters
		if qualifiedInstance.Status.DeletionPhase == api.DeleteSpokeChildApps {
			if err := r.deleteSpokeApps(targetApp, app, ns); err != nil {
				return err
			}

			if err := r.updateDeletionPhase(qualifiedInstance, api.DeleteSpoke); err != nil {
				return err
			}

			return fmt.Errorf("all child applications are gone, transitioning to %s phase", api.DeleteSpoke)
		}

		// Phase 2: Delete app of apps from spoke
		if qualifiedInstance.Status.DeletionPhase == api.DeleteSpoke {
			changed, errUpdate := updateApplication(r.argoClient, targetApp, app, ns)
			if errUpdate != nil {
				return fmt.Errorf("failed to update application %q for spoke app of apps deletion: %v", app.Name, errUpdate)
			}
			if changed {
				return fmt.Errorf("updated application %q for spoke app of apps deletion", app.Name)
			}

			if err := syncApplication(r.argoClient, app, false); err != nil {
				return err
			}

			childApps, err := getChildApplications(r.argoClient, app)
			if err != nil {
				return err
			}

			// We need to prune policies from acm, to initiate app of apps removal from spoke
			for _, childApp := range childApps { //nolint:gocritic // rangeValCopy: each iteration copies 992 bytes
				if err := syncApplication(r.argoClient, &childApp, true); err != nil {
					return err
				}
			}

			// Check if app of apps are gone from spoke
			if _, err = r.checkSpokeApplicationsGone(true); err != nil {
				return fmt.Errorf("error checking applications: %w", err)
			}

			if err := r.updateDeletionPhase(qualifiedInstance, api.DeleteHubChildApps); err != nil {
				return err
			}

			return fmt.Errorf("app of apps are gone from spokes, transitioning to %s phase", api.DeleteHubChildApps)
		}

		// Phase 3: Delete applications from hub
		if qualifiedInstance.Status.DeletionPhase == api.DeleteHubChildApps {
			if err := r.deleteHubApps(targetApp, app, ns); err != nil {
				return err
			}

			if err := r.updateDeletionPhase(qualifiedInstance, api.DeleteHub); err != nil {
				return err
			}

			return fmt.Errorf("apps are gone from hub, transitioning to %s phase", api.DeleteHub)
		}
		// Phase 4: Delete app of apps from hub
		if qualifiedInstance.Status.DeletionPhase == api.DeleteHub {
			log.Printf("removing the application, and cascading to anything instantiated by ArgoCD")
			if err := removeApplication(r.argoClient, app.Name, ns); err != nil {
				return err
			}
			// Clean up the ConsoleLink if we created one
			if !isLegacyArgoNamespace() {
				err := removeConsoleLink(r.dynamicClient, getClusterWideArgoName())
				if err != nil {
					log.Printf("failed to remove the consoleLink: %v", err)
				}
			}
		}
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
	r.gitOperations = &GitOperationsImpl{}
	r.giteaOperations = &GiteaOperationsImpl{}
	r.mgr = mgr

	var ctrlErr error
	r.ctrl, ctrlErr = ctrl.NewControllerManagedBy(mgr).
		For(&api.Pattern{}).
		Build(r)
	return ctrlErr
}

// startArgoCDWatch dynamically adds a watch on ArgoCD instances so that if
// the ArgoCD CR is deleted (e.g. during an upgrade that changes the GitOps
// Subscription), the Pattern controller reconciles immediately and recreates it.
// This is called from the reconcile loop once the GitOps operator has been
// installed and the ArgoCD CRD is available.
func (r *PatternReconciler) startArgoCDWatch() {
	if r.argoCDWatchStarted {
		return
	}

	if err := checkAPIVersion(r.fullClient, ArgoCDGroup, ArgoCDVersion); err != nil {
		return
	}

	argoCD := &unstructured.Unstructured{}
	argoCD.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   ArgoCDGroup,
		Version: ArgoCDVersion,
		Kind:    "ArgoCD",
	})

	err := r.ctrl.Watch(source.Kind(r.mgr.GetCache(), argoCD,
		handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj *unstructured.Unstructured) []reconcile.Request {
			if obj.GetName() != getClusterWideArgoName() || obj.GetNamespace() != getClusterWideArgoNamespace() {
				return nil
			}
			var patterns api.PatternList
			if err := r.mgr.GetClient().List(ctx, &patterns); err != nil {
				return nil
			}
			requests := make([]reconcile.Request, len(patterns.Items))
			for i := range patterns.Items {
				requests[i] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      patterns.Items[i].Name,
						Namespace: patterns.Items[i].Namespace,
					},
				}
			}
			return requests
		}),
	))
	if err != nil {
		ctrl.Log.Error(err, "Failed to start ArgoCD watch, will retry on next reconcile")
		return
	}

	ctrl.Log.Info("ArgoCD watch started successfully")
	r.argoCDWatchStarted = true
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
		// Return nil error when we have a duration to avoid exponential backoff
		return reconcile.Result{RequeueAfter: *duration}, nil
	}
	return reconcile.Result{}, err
}

// detectArgoNamespace determines whether this is a legacy upgrade or greenfield deploy
// by checking if the legacy ArgoCD CR exists. Sets the package-level active ArgoCD
// namespace/name accordingly.
func detectArgoNamespace(dynamicClient dynamic.Interface) {
	if haveArgo(dynamicClient, LegacyClusterWideArgoName, LegacyApplicationNamespace) {
		activeArgoNamespace = LegacyApplicationNamespace
		activeArgoName = LegacyClusterWideArgoName
		logOnce(fmt.Sprintf("Detected legacy ArgoCD instance, using namespace: %s", LegacyApplicationNamespace))
	} else {
		activeArgoNamespace = ApplicationNamespace
		activeArgoName = ClusterWideArgoName
		logOnce(fmt.Sprintf("No legacy ArgoCD instance found, using namespace: %s", ApplicationNamespace))
	}
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

// checkSpokeApplicationsGone checks if all applications are gone from spoke clusters
// passing appOfApps true will check the app of app instead of child apps
// The operator runs on the hub cluster and needs to check spoke clusters through ACM Search Service
// Returns true if all child applications are gone, false otherwise
func (r *PatternReconciler) checkSpokeApplicationsGone(appOfApps bool) (bool, error) {
	// Running locally: use localhost with env var set to "https://localhost:4010/searchapi/graphql" and port-forward
	// User should run: kubectl port-forward -n open-cluster-management svc/search-search-api 4010:4010
	searchURL := os.Getenv("ACM_SEARCH_API_URL")
	if searchURL == "" {
		searchNamespace := "open-cluster-management" // Default namespace for ACM
		searchURL = fmt.Sprintf("https://search-search-api.%s.svc.cluster.local:4010/searchapi/graphql", searchNamespace)
	}

	parsedURL, err := url.Parse(searchURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return false, fmt.Errorf("invalid search API URL: %s", searchURL)
	}
	cleanURL := url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   parsedURL.Path,
	}

	token := os.Getenv("ACM_SEARCH_API_TOKEN")
	if token == "" {
		var tokenBytes []byte
		var err error

		tokenPath := "/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec

		if tokenBytes, err = os.ReadFile(tokenPath); err != nil {
			return false, fmt.Errorf("failed to read serviceaccount token: %w", err)
		}
		token = string(tokenBytes)
	}

	// Build GraphQL query to search for Applications
	// Filter out local-cluster apps and app of apps (based on namespace)
	ns := []string{fmt.Sprintf("!%s", getClusterWideArgoNamespace())}
	if appOfApps {
		ns = []string{getClusterWideArgoNamespace()}
	}
	query := map[string]any{
		"operationName": "searchResult",
		"query":         "query searchResult($input: [SearchInput]) { searchResult: search(input: $input) { items related { kind items } } }",
		"variables": map[string]any{
			"input": []map[string]any{
				{
					"filters": []map[string]any{
						{
							"property": "apigroup",
							"values":   []string{"argoproj.io"},
						},
						{
							"property": "kind",
							"values":   []string{"Application"},
						},
						{
							"property": "cluster",
							"values":   []string{"!local-cluster"},
						},
						{
							"property": "namespace",
							"values":   ns,
						},
					},
					"relatedKinds": []string{"Application"},
					"limit":        20000,
				},
			},
		},
	}

	// Marshal query to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return false, fmt.Errorf("failed to marshal GraphQL query: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "POST", cleanURL.String(), bytes.NewBuffer(queryJSON))
	if err != nil {
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Create HTTP client
	// Use insecure TLS (self-signed certs)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	// Make the request
	resp, err := httpClient.Do(req) //nolint:gosec // URL is constructed from a known internal service endpoint
	if err != nil {
		return false, fmt.Errorf("failed to make HTTP request to search service: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("search service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	type SearchAPIResponse struct {
		Data struct {
			SearchResult []struct {
				Items []struct {
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
					Cluster   string `json:"cluster"`
				} `json:"items"`
			} `json:"searchResult"`
		} `json:"data"`
	}
	var searchResponse SearchAPIResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return false, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var remote_app_names []string
	if searchResult := searchResponse.Data.SearchResult; len(searchResult) > 0 {
		for _, item := range searchResult[0].Items {
			remote_app_names = append(remote_app_names, fmt.Sprintf("%s/%s in %s", item.Namespace, item.Name, item.Cluster))
		}
	}

	if len(remote_app_names) != 0 {
		return false, fmt.Errorf("spoke cluster apps still exist: %s", remote_app_names)
	}

	return true, nil
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
