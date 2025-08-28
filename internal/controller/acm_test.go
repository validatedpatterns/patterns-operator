package controllers

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveACMHub", func() {
	var (
		patternReconciler *PatternReconciler
		dynamicClient     *dynamicfake.FakeDynamicClient
		gvrMCH            schema.GroupVersionResource
		fakeClient        client.Client
	)

	BeforeEach(func() {
		gvrMCH = schema.GroupVersionResource{Group: "operator.open-cluster-management.io", Version: "v1", Resource: "multiclusterhubs"}

		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvrMCH: "MultiClusterHubList",
		})

	})

	Context("when the ACM Hub exists", func() {
		BeforeEach(func() {
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
					Labels: map[string]string{
						"ocm-configmap-type": "image-manifest",
					},
				},
			}

			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()
			patternReconciler = &PatternReconciler{
				Client:        fakeClient,
				dynamicClient: dynamicClient,
			}

			hub := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "operator.open-cluster-management.io/v1",
					"kind":       "MultiClusterHub",
					"metadata": map[string]any{
						"name":      "test-hub",
						"namespace": "default",
					},
				},
			}
			_, err := dynamicClient.Resource(gvrMCH).Namespace("default").Create(context.Background(), hub, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the ACM Hub does not exist", func() {
		It("should return false", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()
			patternReconciler = &PatternReconciler{
				Client:        fakeClient,
				dynamicClient: dynamicClient,
			}

			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error listing ConfigMaps", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{List: func(ctx context.Context, client client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("list error")
				}}).WithScheme(testEnv.Scheme).Build()
			patternReconciler = &PatternReconciler{
				Client:        fakeClient,
				dynamicClient: dynamicClient,
			}
		})

		It("should return false and log the error", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error listing the MultiClusterHubs", func() {
		BeforeEach(func() {
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
					Labels: map[string]string{
						"ocm-configmap-type": "image-manifest",
					},
				},
			}
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(configMap).Build()

			dynamicClient.PrependReactor("list", "multiclusterhubs", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("multiclusterhub error")
			})
			patternReconciler = &PatternReconciler{
				Client:        fakeClient,
				dynamicClient: dynamicClient,
			}
		})

		It("should return false and log the error", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})
})
