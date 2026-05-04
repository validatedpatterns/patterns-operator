package console

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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

var _ = Describe("CreateOrUpdateCatalog", func() {
	var (
		ctx context.Context
		cm  *corev1.ConfigMap
		cl  client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("with default image", func() {
		BeforeEach(func() {
			cm = newOperatorConfigMap("")
			cl = newFakeClient(cm)
		})

		It("should create a deployment with the default image", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal(CatalogDefaultImage))
		})
	})

	Context("with an overridden image", func() {
		BeforeEach(func() {
			cm = newOperatorConfigMap("custom-catalog:v2")
			cl = newFakeClient(cm)
		})

		It("should create a deployment with the overridden image", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal("custom-catalog:v2"))
		})
	})

	Context("when updating an existing deployment", func() {
		BeforeEach(func() {
			cm = newOperatorConfigMap("custom-catalog:v3")
			cl = newFakeClient(cm)
		})

		It("should update the deployment image", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			// Change the override
			cm.Data["catalog.image"] = "custom-catalog:v4"
			Expect(cl.Update(ctx, cm)).To(Succeed())

			// Second call updates
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal("custom-catalog:v4"))
		})
	})

	Context("when checking auxiliary resources", func() {
		BeforeEach(func() {
			cm = newOperatorConfigMap("")
			cl = newFakeClient(cm)
		})

		It("should create the nginx ConfigMap", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			catalogCM := &corev1.ConfigMap{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogConfigMapName}, catalogCM)).To(Succeed())
			Expect(catalogCM.Data).To(HaveKey("nginx.conf"))
		})

		It("should create the Service with the correct port", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, cm)).To(Succeed())

			svc := &corev1.Service{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogServiceName}, svc)).To(Succeed())
			Expect(svc.Spec.Ports[0].Port).To(BeNumerically("==", PatternCatalogServicePort))
		})
	})

	Context("when the operator ConfigMap is missing", func() {
		BeforeEach(func() {
			cl = newFakeClient()
		})

		It("should fall back to the default image", func() {
			Expect(CreateOrUpdateCatalog(ctx, cl, nil)).To(Succeed())

			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal(CatalogDefaultImage))
		})
	})

	Context("when the operator ConfigMap is empty (no data)", func() {
		BeforeEach(func() {
			cl = newFakeClient()

		})

		It("should fall back to the default image", func() {
			configmap := &corev1.ConfigMap{}

			Expect(CreateOrUpdateCatalog(ctx, cl, configmap)).To(Succeed())

			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: CatalogDeploymentName}, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal(CatalogDefaultImage))
		})
	})
})
