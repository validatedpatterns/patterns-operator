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
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	"github.com/hybrid-cloud-patterns/patterns-operator/controllers"
	"github.com/hybrid-cloud-patterns/patterns-operator/version"
	routev1 "github.com/openshift/api/route/v1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(gitopsv1alpha1.AddToScheme(scheme))
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
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f2850479.hybrid-cloud-patterns.io",
		//LeaderElectionNamespace: "default", // Use this if we ever want to enforce a single instance per cluster
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	isAnalyticsDisabled := strings.ToLower(os.Getenv("ANALYTICS")) == "false"
	if err = (&controllers.PatternReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		AnalyticsClient: controllers.AnalyticsInit(isAnalyticsDisabled, setupLog),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pattern")
		os.Exit(1)
	}
	if err = (&controllers.GiteaServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GiteaServer")
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
	// Setup Scheme for OpenShift Routes if available.
	if err := routev1.Install(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
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
		return fmt.Errorf("Failed to get config: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Failed to call NewForConfig: %s", err)
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
