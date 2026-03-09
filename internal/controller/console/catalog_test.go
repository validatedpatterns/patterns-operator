package console

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func clientKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

func newConfigMap(image string) *corev1.ConfigMap {
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

func newCatalogDeployment(image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CatalogDeploymentName,
			Namespace: defaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "catalog"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "catalog"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  CatalogContainerName,
							Image: image,
						},
					},
				},
			},
		},
	}
}

func TestUpdateCatalogImageIfOverridden_NoOverride(t *testing.T) {
	cm := newConfigMap("")
	deploy := newCatalogDeployment("original:latest")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, deploy).Build()

	if err := UpdateCatalogImageIfOverridden(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), clientKey(defaultNamespace, CatalogDeploymentName), updated); err != nil {
		t.Fatalf("unexpected error getting deployment: %v", err)
	}

	got := updated.Spec.Template.Spec.Containers[0].Image
	if got != "original:latest" {
		t.Errorf("expected image to remain 'original:latest', got %q", got)
	}
}

func TestUpdateCatalogImageIfOverridden_WithOverride(t *testing.T) {
	cm := newConfigMap("custom-catalog:v2")
	deploy := newCatalogDeployment("original:latest")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, deploy).Build()

	if err := UpdateCatalogImageIfOverridden(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), clientKey(defaultNamespace, CatalogDeploymentName), updated); err != nil {
		t.Fatalf("unexpected error getting deployment: %v", err)
	}

	got := updated.Spec.Template.Spec.Containers[0].Image
	if got != "custom-catalog:v2" {
		t.Errorf("expected image to be 'custom-catalog:v2', got %q", got)
	}
}

func TestUpdateCatalogImageIfOverridden_AlreadyUpToDate(t *testing.T) {
	cm := newConfigMap("custom-catalog:v2")
	deploy := newCatalogDeployment("custom-catalog:v2")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm, deploy).Build()

	if err := UpdateCatalogImageIfOverridden(context.Background(), cl, cl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), clientKey(defaultNamespace, CatalogDeploymentName), updated); err != nil {
		t.Fatalf("unexpected error getting deployment: %v", err)
	}

	got := updated.Spec.Template.Spec.Containers[0].Image
	if got != "custom-catalog:v2" {
		t.Errorf("expected image to remain 'custom-catalog:v2', got %q", got)
	}
}

func TestUpdateCatalogImageIfOverridden_MissingConfigMap(t *testing.T) {
	deploy := newCatalogDeployment("original:latest")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy).Build()

	err := UpdateCatalogImageIfOverridden(context.Background(), cl, cl)
	if err == nil {
		t.Fatal("expected error when configmap is missing, got nil")
	}
}

func TestUpdateCatalogImageIfOverridden_MissingDeployment(t *testing.T) {
	cm := newConfigMap("custom-catalog:v2")

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	err := UpdateCatalogImageIfOverridden(context.Background(), cl, cl)
	if err == nil {
		t.Fatal("expected error when deployment is missing, got nil")
	}
}
