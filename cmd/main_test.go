package main

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	controllers "github.com/hybrid-cloud-patterns/patterns-operator/internal/controller"
)

func newFakeReader(objs ...crclient.Object) crclient.Reader {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objs) > 0 {
		builder = builder.WithObjects(objs...)
	}
	return builder.Build()
}

func newOperatorConfigMap(analyticsDisabled string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controllers.OperatorNamespace,
			Name:      controllers.OperatorConfigMap,
		},
		Data: map[string]string{
			"analytics.disabled": analyticsDisabled,
		},
	}
}

func TestIsAnalyticsDisabledWithReader_ConfigMapTrue(t *testing.T) {
	cm := newOperatorConfigMap("true")
	reader := newFakeReader(cm)

	if got := isAnalyticsDisabled(reader); got != true {
		t.Fatalf("expected true when configmap sets analytics.disabled=true, got %v", got)
	}
}

func TestIsAnalyticsDisabledWithReader_ConfigMapFalse(t *testing.T) {
	cm := newOperatorConfigMap("false")
	reader := newFakeReader(cm)

	if got := isAnalyticsDisabled(reader); got != false {
		t.Fatalf("expected false when configmap sets analytics.disabled=false, got %v", got)
	}
}

func TestIsAnalyticsDisabledWithReader_NoConfigMap_EnvFalseDisables(t *testing.T) {
	_ = os.Setenv("ANALYTICS", "false")
	t.Cleanup(func() { _ = os.Unsetenv("ANALYTICS") })

	reader := newFakeReader()
	if got := isAnalyticsDisabled(reader); got != true {
		t.Fatalf("expected true when no configmap and ANALYTICS=false, got %v", got)
	}
}

func TestIsAnalyticsDisabledWithReader_NoConfigMap_EnvTrueEnables(t *testing.T) {
	_ = os.Setenv("ANALYTICS", "true")
	t.Cleanup(func() { _ = os.Unsetenv("ANALYTICS") })

	reader := newFakeReader()
	if got := isAnalyticsDisabled(reader); got != false {
		t.Fatalf("expected false when no configmap and ANALYTICS=true, got %v", got)
	}
}
