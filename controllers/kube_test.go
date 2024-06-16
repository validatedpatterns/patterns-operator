package controllers

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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

	Context("when the API versions are invalid", func() {
		BeforeEach(func() {
			refA = &metav1.OwnerReference{
				APIVersion: "invalid/v1/v2",
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
