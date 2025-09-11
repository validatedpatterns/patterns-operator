package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveACMHub", func() {

	var (
		patternReconciler *PatternReconciler
		fakeClient        client.Client
		configMap         *v1.ConfigMap
		hub               *unstructured.Unstructured
	)
	BeforeEach(func() {
		configMap = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-configmap",
				Namespace: "default",
				Labels: map[string]string{
					"ocm-configmap-type": "image-manifest",
				},
			},
		}
		hub = &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "operator.open-cluster-management.io/v1",
				"kind":       "MultiClusterHub",
				"metadata": map[string]any{
					"name":      "test-hub",
					"namespace": "default",
				},
			},
		}
		hub.SetGroupVersionKind(mchGVK)
	})

	Context("when the ACM Hub exists in same ns as configmap", func() {
		It("should return true", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap, hub).Build()
			patternReconciler = &PatternReconciler{
				Client: fakeClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the ACM Hub exists different ns as configmap", func() {
		It("should return false", func() {
			hub.SetNamespace("different")

			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap, hub).Build()
			patternReconciler = &PatternReconciler{
				Client: fakeClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the ACM Hub does not exist", func() {
		It("should return false", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()
			patternReconciler = &PatternReconciler{
				Client: fakeClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error listing ConfigMaps", func() {
		It("should return false and log the error", func() {
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{List: func(ctx context.Context, client client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("list error")
				}}).WithScheme(testEnv.Scheme).Build()

			patternReconciler = &PatternReconciler{
				Client: fakeClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error listing the MultiClusterHubs", func() {
		It("should return false and log the error", func() {
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("list error")
				}}).WithScheme(testEnv.Scheme).WithRuntimeObjects(configMap).Build()

			patternReconciler = &PatternReconciler{
				Client: fakeClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})
})
