package controllers

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveNamespace", func() {
	var (
		controllerClient client.Client
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
		fakeClient client.Client
		namespace  string
		routeName  string
	)

	BeforeEach(func() {
		namespace = "default"
		routeName = "test-route"
	})

	Context("when the route exists", func() {
		It("should return the URL of the route", func() {
			route := routev1.Route{
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

			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(&route).Build()

			url, err := getRoute(fakeClient, routeName, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(url).To(Equal("https://example.com"))
		})
	})

	Context("when the route does not exist", func() {
		It("should return an error", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()

			url, err := getRoute(fakeClient, routeName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(url).To(BeEmpty())
		})
	})

	Context("when the route has no ingress", func() {
		It("should return an error", func() {
			route := routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeName,
					Namespace: namespace,
				},
				Status: routev1.RouteStatus{},
			}

			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(&route).Build()

			url, err := getRoute(fakeClient, routeName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(url).To(BeEmpty())
		})
	})
})

var _ = Describe("GetSecret", func() {
	var (
		fakeClient client.Client
		namespace  string
		secretName string
		secret     *v1.Secret
	)

	BeforeEach(func() {
		namespace = "default"
		secretName = "test-secret"
		secret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		}
	})

	Context("when the secret exists", func() {
		BeforeEach(func() {

		})

		It("should return the secret", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects(secret).Build()

			secret, err := getSecret(fakeClient, secretName, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).ToNot(BeNil())
			Expect(secret.Name).To(Equal(secretName))
		})
	})

	Context("when the secret does not exist", func() {
		It("should return an error", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testEnv.Scheme).
				WithRuntimeObjects().Build()

			secret, err := getSecret(fakeClient, secretName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(secret).To(BeNil())
		})
	})

	Context("when there is an error other than NotFound", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithInterceptorFuncs(
				interceptor.Funcs{
					Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.NewInternalError(fmt.Errorf("internal error"))
					},
				}).WithScheme(testEnv.Scheme).Build()

		})

		It("should return an error", func() {
			secret, err := getSecret(fakeClient, secretName, namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInternalError(err)).To(BeTrue())
			Expect(secret).To(BeNil())
		})
	})
})
