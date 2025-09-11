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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	argoapi "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	controllers "github.com/hybrid-cloud-patterns/patterns-operator/internal/controller"
	"github.com/hybrid-cloud-patterns/patterns-operator/version"
	apiv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(gitopsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiv1.Install(scheme))
	utilruntime.Must(operatorv1.Install(scheme))
	utilruntime.Must(argoapi.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	printVersion()

	// Create initial config map for gitops
	err := createGitOpsConfigMap()
	if err != nil {
		setupLog.Error(err, "unable to create config map")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f2850479.hybrid-cloud-patterns.io",
		//LeaderElectionNamespace: "default", // Use this if we ever want to enforce a single instance per cluster
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	registerComponentOrExit(mgr, argov1beta1api.AddToScheme)

	analyticsEnabled := areAnalyticsEnabled(mgr.GetAPIReader())
	setupLog.Info("analytics enabled", "enabled", analyticsEnabled)
	if err = (&controllers.PatternReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		AnalyticsClient: controllers.AnalyticsInit(!analyticsEnabled, setupLog),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pattern")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: v%s", runtime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	setupLog.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	setupLog.Info(fmt.Sprintf("Git Commit: %s", version.GitCommit))
	setupLog.Info(fmt.Sprintf("Build Date: %s", version.BuildDate))
}

// Creates the patterns operator configmap
// This will include configuration parameters that
// will allow operator configuration
func createGitOpsConfigMap() error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to call NewForConfig: %s", err)
	}

	configMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.OperatorConfigMap,
			Namespace: controllers.OperatorNamespace,
		},
	}

	_, err = clientset.CoreV1().ConfigMaps(controllers.OperatorNamespace).Get(context.Background(), controllers.OperatorConfigMap, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// if the configmap does not exist we create an empty one
		_, err = clientset.CoreV1().ConfigMaps(controllers.OperatorNamespace).Create(context.Background(), &configMap, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		// if we had an error that is not IsNotFound we need to return it
		return err
	}
	return nil
}

func registerComponentOrExit(mgr manager.Manager, f func(*k8sruntime.Scheme) error) {
	// Setup Scheme for all resources
	if err := f(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
	setupLog.Info(fmt.Sprintf("Component registered: %v", reflect.ValueOf(f)))
}

// areAnalyticsEnabled determines whether analytics are enabled.
// Precedence: Operator ConfigMap key "analytics.enabled" (true/false) > ENV ANALYTICS (false means disabled)
func areAnalyticsEnabled(reader crclient.Reader) bool {
	enabled := strings.ToLower(os.Getenv("ANALYTICS")) != "false"

	var cm corev1.ConfigMap
	err := reader.Get(context.Background(), crclient.ObjectKey{Namespace: controllers.OperatorNamespace, Name: controllers.OperatorConfigMap}, &cm)
	if err != nil {
		setupLog.Error(err, "error reading operator configmap for analytics setting")
		return enabled
	}

	if v, ok := cm.Data["analytics.enabled"]; ok {
		return strings.EqualFold(v, "true")
	}

	return enabled
}
