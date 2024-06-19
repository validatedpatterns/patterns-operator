package controllers

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	routefake "github.com/openshift/client-go/route/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	kubeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveNamespace", func() {
	var (
		controllerClient kubeclient.Client
		namespaceName    string
	)

	BeforeEach(func() {
		namespaceName = "test-namespace"
		s := scheme.Scheme
		s.AddKnownTypes(v1.SchemeGroupVersion, &v1.Namespace{})
		controllerClient = fake.NewClientBuilder().WithScheme(s).Build()
	})

	Context("when the namespace exists", func() {
		BeforeEach(func() {
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			err := controllerClient.Create(context.Background(), ns)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true", func() {
			exists := haveNamespace(controllerClient, namespaceName)
			Expect(exists).To(BeTrue())
		})
	})

	Context("when the namespace does not exist", func() {
		It("should return false", func() {
			exists := haveNamespace(controllerClient, namespaceName)
			Expect(exists).To(BeFalse())
		})
	})
})

var _ = Describe("OwnedBySame", func() {
	var (
		expected metav1.Object
		object   metav1.Object
	)

	BeforeEach(func() {
		expected = &metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID: "owner-uid-1",
				},
				{
					UID: "owner-uid-2",
				},
			},
		}
	})

	Context("when both objects have the same owner references", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						UID: "owner-uid-1",
					},
					{
						UID: "owner-uid-2",
					},
				},
			}
		})

		It("should return true", func() {
			result := ownedBySame(expected, object)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the objects have different owner references", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						UID: "owner-uid-3",
					},
				},
			}
		})

		It("should return false", func() {
			result := ownedBySame(expected, object)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the object has additional owner references", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						UID: "owner-uid-1",
					},
					{
						UID: "owner-uid-2",
					},
					{
						UID: "owner-uid-3",
					},
				},
			}
		})

		It("should return true", func() {
			result := ownedBySame(expected, object)
			Expect(result).To(BeTrue())
		})
	})

	Context("when object has no owner references", func() {
		BeforeEach(func() {
			expected = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						UID: "owner-uid-1",
					},
				},
			}
			object = &metav1.ObjectMeta{}
		})

		It("should return false", func() {
			result := ownedBySame(expected, object)
			Expect(result).To(BeFalse())
		})
	})

	Context("when both objects have no owner references", func() {
		BeforeEach(func() {
			expected = &metav1.ObjectMeta{}
			object = &metav1.ObjectMeta{}
		})

		It("should return true", func() {
			result := ownedBySame(expected, object)
			Expect(result).To(BeTrue())
		})
	})
})

var _ = Describe("ReferSameObject", func() {
	var (
		refA *metav1.OwnerReference
		refB *metav1.OwnerReference
	)

	Context("when both references point to the same object", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
		})

		It("should return true", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the API versions are different", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "v2",
				Kind:       "Pod",
				Name:       "mypod",
			}
		})

		It("should return false", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the kinds are different", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Service",
				Name:       "mypod",
			}
		})

		It("should return false", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the names are different", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "yourpod",
			}
		})

		It("should return false", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the first API version is invalid", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "invalid/v1/v2",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "valid/v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
		})

		It("should return false", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the second API version is invalid", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "valid/v1",
				Kind:       "Pod",
				Name:       "mypod",
			}
			refB = &metav1.OwnerReference{
				APIVersion: "invalid/v1/v2",
				Kind:       "Pod",
				Name:       "mypod",
			}
		})

		It("should return false", func() {
			result := referSameObject(refA, refB)
			Expect(result).To(BeFalse())
		})
	})

})

var _ = Describe("OwnedBy", func() {
	var (
		object metav1.Object
		ref    *metav1.OwnerReference
	)

	Context("when the object is owned by the specified owner reference", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
				UID:        "uid-12345",
			}
		})

		It("should return true", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the object is not owned by the specified owner reference", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Service",
				Name:       "myservice",
				UID:        "uid-67890",
			}
		})

		It("should return false", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the owner references have different API versions", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v2",
				Kind:       "Pod",
				Name:       "mypod",
				UID:        "uid-12345",
			}
		})

		It("should return false", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the owner references have different kinds", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Service",
				Name:       "mypod",
				UID:        "uid-12345",
			}
		})

		It("should return false", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the owner references have different names", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "yourpod",
				UID:        "uid-12345",
			}
		})

		It("should return false", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the owner references have different UIDs", func() {
		BeforeEach(func() {
			object = &metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       "mypod",
						UID:        "uid-12345",
					},
				},
			}
			ref = &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "mypod",
				UID:        "uid-67890",
			}
		})

		It("should return false", func() {
			result := ownedBy(object, ref)
			Expect(result).To(BeFalse())
		})
	})
})

type TestStruct struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

var _ = Describe("ObjectYaml", func() {
	Context("when the object can be marshaled to YAML", func() {
		It("should return the correct YAML string", func() {
			obj := TestStruct{
				Name:  "test-name",
				Value: 42,
			}

			yamlString, err := objectYaml(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(yamlString).To(Equal("name: test-name\nvalue: 42\n"))
		})
	})

	// Commented out as the yaml pkg does not detect this and we end up in an OOM loop
	// Context("when the object cannot be marshaled to YAML", func() {
	// 	It("should return an error", func() {
	// 		obj := &struct {
	// 			A string
	// 			B interface{}
	// 		}{
	// 			A: "a string",
	// 		}
	// 		// Add a cycle
	// 		obj.B = obj
	// 		yamlString, err := objectYaml(obj)
	// 		Expect(err).To(HaveOccurred())
	// 		Expect(yamlString).To(BeEmpty())
	// 		Expect(err.Error()).To(ContainSubstring("error marshaling object"))
	// 	})
	// })
})

var _ = Describe("GetRoute", func() {
	var (
		routeClient *routefake.Clientset
		namespace   string
		routeName   string
	)

	BeforeEach(func() {
		routeClient = routefake.NewSimpleClientset()
		namespace = "default"
		routeName = "test-route"
	})

	Context("when the route exists", func() {
		BeforeEach(func() {
			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: namespace,
				},
				Status: routev1.RouteStatus{
					Ingress: []routev1.RouteIngress{
						{
							Host: "example.com",
						},
					},
				},
			}
			_, err := routeClient.RouteV1().Routes(namespace).Create(context.Background(), route, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the URL of the route", func() {
			url, err := getRoute(routeClient, routeName, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(url).To(Equal("https://example.com"))
		})
	})

	Context("when the route does not exist", func() {
		It("should return an error", func() {
			url, err := getRoute(routeClient, routeName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(url).To(BeEmpty())
		})
	})

	Context("when the route has no ingress", func() {
		BeforeEach(func() {
			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: namespace,
				},
				Status: routev1.RouteStatus{},
			}
			_, err := routeClient.RouteV1().Routes(namespace).Create(context.Background(), route, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error", func() {
			url, err := getRoute(routeClient, routeName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(url).To(BeEmpty())
		})
	})
})

var _ = Describe("GetSecret", func() {
	var (
		clientset  *kubefake.Clientset
		namespace  string
		secretName string
	)

	BeforeEach(func() {
		clientset = kubefake.NewSimpleClientset()
		namespace = "default"
		secretName = "test-secret"
	})

	Context("when the secret exists", func() {
		BeforeEach(func() {
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
			}
			_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the secret", func() {
			secret, err := getSecret(clientset, secretName, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).ToNot(BeNil())
			Expect(secret.Name).To(Equal(secretName))
		})
	})

	Context("when the secret does not exist", func() {
		It("should return an error", func() {
			secret, err := getSecret(clientset, secretName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(secret).To(BeNil())
		})
	})

	Context("when there is an error other than NotFound", func() {
		BeforeEach(func() {
			clientset.PrependReactor("get", "secrets", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.NewInternalError(fmt.Errorf("internal error"))
			})
		})

		It("should return an error", func() {
			secret, err := getSecret(clientset, secretName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInternalError(err)).To(BeTrue())
			Expect(secret).To(BeNil())
		})
	})
})

// CustomClientset is a wrapper around fake.Clientset that overrides the Discovery method
type CustomClientset struct {
	*kubefake.Clientset
	discovery *discoveryfake.FakeDiscovery
}

func (c *CustomClientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

var _ = Describe("checkAPIVersion", func() {
	var (
		clientset *CustomClientset
	)

	BeforeEach(func() {
		clientset = &CustomClientset{
			Clientset: kubefake.NewSimpleClientset(),
			discovery: &discoveryfake.FakeDiscovery{
				Fake: &kubefake.NewSimpleClientset().Fake},
		}
	})

	It("should return an error when the API group and version do not exist", func() {
		err := checkAPIVersion(clientset, ArgoCDGroup, ArgoCDVersion)
		Expect(err).To(Not(BeNil()))
		Expect(err).To(MatchError(fmt.Sprintf("API version %s/%s not available", ArgoCDGroup, ArgoCDVersion)))
	})

	It("should return nil when the API group and version exist", func() {
		clientset.discovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: fmt.Sprintf("%s/%s", ArgoCDGroup, ArgoCDVersion),
				APIResources: []metav1.APIResource{},
			},
		}

		err := checkAPIVersion(clientset, ArgoCDGroup, ArgoCDVersion)
		Expect(err).To(BeNil())
	})

	It("should return an error when the API group exists but the version does not", func() {
		clientset.discovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: fmt.Sprintf("%s/%s", ArgoCDGroup, "v10"),
				APIResources: []metav1.APIResource{},
			},
		}

		err := checkAPIVersion(clientset, ArgoCDGroup, ArgoCDVersion)
		Expect(err).To(MatchError(fmt.Sprintf("API version %s/%s not available", ArgoCDGroup, ArgoCDVersion)))
	})

	// FIXME(bandini): Not working yet
	// It("should return an error when there is an error fetching the API groups", func() {
	// 	clientset.discovery.PrependReactor("*", "*", func(testing.Action) (handled bool, ret runtime.Object, err error) {
	// 		return true, nil, kubeerrors.NewInternalError(fmt.Errorf("discovery error"))
	// 	})

	// 	err := checkAPIVersion(clientset, "example.com", "v1")
	// 	Expect(err).To(MatchError("failed to get API groups: discovery error"))
	// })
})
