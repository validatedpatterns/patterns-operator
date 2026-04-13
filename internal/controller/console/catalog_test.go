package console

import (
	"context"
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMain(m *testing.M) {
	// Ensure getDeploymentNamespace() always returns defaultNamespace in tests,
	// regardless of whether /var/run/secrets/.../namespace exists (e.g. in CI).
	os.Setenv("OPERATOR_NAMESPACE", defaultNamespace)
	os.Exit(m.Run())
}

func newOperatorConfigMap(image string) *corev1.ConfigMap {
	data := map[string]string{}
	if image != "" {
		data["catalog.image"] = image
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorConfigMap,
			Namespace: defaultNamespace,
		},
		Data: data,
	}
}

func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestCreateOrUpdateCatalog_DefaultImage(t *testing.T) {
	cm := newOperatorConfigMap("")
	cl := newFakeClient(cm)

	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy); err != nil {
		t.Fatalf("expected catalog deployment to be created: %v", err)
	}

	got := deploy.Spec.Template.Spec.Containers[0].Image
	if got != CatalogDefaultImage {
		t.Errorf("expected image %q, got %q", CatalogDefaultImage, got)
	}
}

func TestCreateOrUpdateCatalog_OverriddenImage(t *testing.T) {
	cm := newOperatorConfigMap("custom-catalog:v2")
	cl := newFakeClient(cm)

	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy); err != nil {
		t.Fatalf("expected catalog deployment to be created: %v", err)
	}

	got := deploy.Spec.Template.Spec.Containers[0].Image
	if got != "custom-catalog:v2" {
		t.Errorf("expected image %q, got %q", "custom-catalog:v2", got)
	}
}

func TestCreateOrUpdateCatalog_UpdatesExistingDeployment(t *testing.T) {
	cm := newOperatorConfigMap("custom-catalog:v3")
	cl := newFakeClient(cm)

	// First call creates
	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error on create: %v", err)
	}

	// Change the override
	cm.Data["catalog.image"] = "custom-catalog:v4"
	if err := cl.Update(context.Background(), cm); err != nil {
		t.Fatalf("unexpected error updating configmap: %v", err)
	}

	// Second call updates
	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy); err != nil {
		t.Fatalf("unexpected error getting deployment: %v", err)
	}

	got := deploy.Spec.Template.Spec.Containers[0].Image
	if got != "custom-catalog:v4" {
		t.Errorf("expected image %q, got %q", "custom-catalog:v4", got)
	}
}

func TestCreateOrUpdateCatalog_CreatesConfigMapAndService(t *testing.T) {
	cm := newOperatorConfigMap("")
	cl := newFakeClient(cm)

	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check ConfigMap
	catalogCM := &corev1.ConfigMap{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogConfigMapName}, catalogCM); err != nil {
		t.Fatalf("expected catalog configmap to be created: %v", err)
	}
	if _, ok := catalogCM.Data["nginx.conf"]; !ok {
		t.Error("expected nginx.conf key in catalog configmap")
	}

	// Check Service
	svc := &corev1.Service{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogServiceName}, svc); err != nil {
		t.Fatalf("expected catalog service to be created: %v", err)
	}
	if svc.Spec.Ports[0].Port != PatternCatalogServicePort {
		t.Errorf("expected service port %d, got %d", PatternCatalogServicePort, svc.Spec.Ports[0].Port)
	}
}

func TestCreateOrUpdateCatalog_MissingOperatorConfigMap(t *testing.T) {
	// No operator configmap — should fall back to default image
	cl := newFakeClient()

	if err := CreateOrUpdateCatalog(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deploy := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy); err != nil {
		t.Fatalf("expected catalog deployment to be created: %v", err)
	}

	got := deploy.Spec.Template.Spec.Containers[0].Image
	if got != CatalogDefaultImage {
		t.Errorf("expected default image %q, got %q", CatalogDefaultImage, got)
	}
}
