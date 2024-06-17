package controllers

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveACMHub", func() {
	var (
		patternReconciler *PatternReconciler
		kubeClient        *fake.Clientset
		dynamicClient     *dynamicfake.FakeDynamicClient
		gvrMCH            schema.GroupVersionResource
	)

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		gvrMCH = schema.GroupVersionResource{Group: "operator.open-cluster-management.io", Version: "v1", Resource: "multiclusterhubs"}

		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvrMCH: "MultiClusterHubList",
		})

		patternReconciler = &PatternReconciler{
			fullClient:    kubeClient,
			dynamicClient: dynamicClient,
		}

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
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

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
			_, err = dynamicClient.Resource(gvrMCH).Namespace("default").Create(context.Background(), hub, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the ACM Hub does not exist", func() {
		It("should return false", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})

	Context("when there is an error listing ConfigMaps", func() {
		BeforeEach(func() {
			kubeClient.PrependReactor("list", "configmaps", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("config map error")
			})
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
			_, err := kubeClient.CoreV1().ConfigMaps("default").Create(context.Background(), configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			dynamicClient.PrependReactor("list", "multiclusterhubs", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("multiclusterhub error")
			})
		})

		It("should return false and log the error", func() {
			result := haveACMHub(patternReconciler)
			Expect(result).To(BeFalse())
		})
	})
})
