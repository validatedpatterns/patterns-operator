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
			hub := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "operator.open-cluster-management.io/v1",
					"kind":       "MultiClusterHub",
					"metadata": map[string]any{
						"name":      "multiclusterhub",
						"namespace": "open-cluster-management",
					},
				},
			}
			_, err := dynamicClient.Resource(gvrMCH).Namespace("open-cluster-management").Create(context.Background(), hub, metav1.CreateOptions{})
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

var _ = Describe("ListManagedClusters", func() {
	var (
		patternReconciler *PatternReconciler
		dynamicClient     *dynamicfake.FakeDynamicClient
		gvrMC             schema.GroupVersionResource
	)

	BeforeEach(func() {
		gvrMC = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}

		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvrMC: "ManagedClusterList",
		})

		patternReconciler = &PatternReconciler{
			dynamicClient: dynamicClient,
		}
	})

	Context("when there are managed clusters", func() {
		BeforeEach(func() {
			for _, name := range []string{"local-cluster", "spoke-1", "spoke-2"} {
				mc := &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "cluster.open-cluster-management.io/v1",
						"kind":       "ManagedCluster",
						"metadata": map[string]any{
							"name": name,
						},
					},
				}
				_, err := dynamicClient.Resource(gvrMC).Create(context.Background(), mc, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should return all clusters except local-cluster", func() {
			clusters, err := patternReconciler.listManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(HaveLen(2))
			Expect(clusters).To(ContainElement("spoke-1"))
			Expect(clusters).To(ContainElement("spoke-2"))
			Expect(clusters).ToNot(ContainElement("local-cluster"))
		})
	})

	Context("when there are no managed clusters", func() {
		It("should return empty list", func() {
			clusters, err := patternReconciler.listManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(BeEmpty())
		})
	})

	Context("when only local-cluster exists", func() {
		BeforeEach(func() {
			mc := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "cluster.open-cluster-management.io/v1",
					"kind":       "ManagedCluster",
					"metadata": map[string]any{
						"name": "local-cluster",
					},
				},
			}
			_, err := dynamicClient.Resource(gvrMC).Create(context.Background(), mc, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return empty list", func() {
			clusters, err := patternReconciler.listManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(clusters).To(BeEmpty())
		})
	})

	Context("when there is an error listing", func() {
		BeforeEach(func() {
			dynamicClient.PrependReactor("list", "managedclusters", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("list error")
			})
		})

		It("should return an error", func() {
			_, err := patternReconciler.listManagedClusters(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list ManagedClusters"))
		})
	})
})

var _ = Describe("DeleteManagedClusters", func() {
	var (
		patternReconciler *PatternReconciler
		dynamicClient     *dynamicfake.FakeDynamicClient
		gvrMC             schema.GroupVersionResource
	)

	BeforeEach(func() {
		gvrMC = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}

		dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
			gvrMC: "ManagedClusterList",
		})

		patternReconciler = &PatternReconciler{
			dynamicClient: dynamicClient,
		}
	})

	Context("when there are managed clusters to delete", func() {
		BeforeEach(func() {
			for _, name := range []string{"local-cluster", "spoke-1", "spoke-2"} {
				mc := &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "cluster.open-cluster-management.io/v1",
						"kind":       "ManagedCluster",
						"metadata": map[string]any{
							"name": name,
						},
					},
				}
				_, err := dynamicClient.Resource(gvrMC).Create(context.Background(), mc, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should delete all clusters except local-cluster", func() {
			count, err := patternReconciler.deleteManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(2))
		})
	})

	Context("when there are no managed clusters", func() {
		It("should return 0", func() {
			count, err := patternReconciler.deleteManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Context("when only local-cluster exists", func() {
		BeforeEach(func() {
			mc := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "cluster.open-cluster-management.io/v1",
					"kind":       "ManagedCluster",
					"metadata": map[string]any{
						"name": "local-cluster",
					},
				},
			}
			_, err := dynamicClient.Resource(gvrMC).Create(context.Background(), mc, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return 0", func() {
			count, err := patternReconciler.deleteManagedClusters(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Context("when listing fails", func() {
		BeforeEach(func() {
			dynamicClient.PrependReactor("list", "managedclusters", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("list error")
			})
		})

		It("should return an error", func() {
			_, err := patternReconciler.deleteManagedClusters(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list ManagedClusters"))
		})
	})

	Context("when delete fails", func() {
		BeforeEach(func() {
			mc := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "cluster.open-cluster-management.io/v1",
					"kind":       "ManagedCluster",
					"metadata": map[string]any{
						"name": "spoke-1",
					},
				},
			}
			_, err := dynamicClient.Resource(gvrMC).Create(context.Background(), mc, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			dynamicClient.PrependReactor("delete", "managedclusters", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("delete error")
			})
		})

		It("should return an error", func() {
			_, err := patternReconciler.deleteManagedClusters(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete ManagedCluster"))
		})
	})
})
