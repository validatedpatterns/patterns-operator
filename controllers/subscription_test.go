package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const subscriptionTestNamespace = "openshift-operators"

var defaultTestSubscription = operatorv1alpha1.Subscription{
	Spec: &operatorv1alpha1.SubscriptionSpec{
		CatalogSource:       "foosource",
		Package:             "foooperator",
		Channel:             "foochannel",
		InstallPlanApproval: operatorv1alpha1.ApprovalAutomatic,
		Config: &operatorv1alpha1.SubscriptionConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			},
		},
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "foosubscription",
		Namespace: subscriptionTestNamespace,
	},
}

var _ = Describe("Subscription Functions", func() {
	Context("getSubscription", func() {
		var testSubscription *operatorv1alpha1.Subscription
		var fakeOlmClientSet *olmclient.Clientset
		BeforeEach(func() {
			testSubscription = defaultTestSubscription.DeepCopy()
			fakeOlmClientSet = olmclient.NewSimpleClientset()
		})
		It("should error out with a non existing a Subscription", func() {
			err := createSubscription(fakeOlmClientSet, testSubscription)
			Expect(err).To(BeNil())
			_, err = getSubscription(fakeOlmClientSet, "foo", "bar")
			Expect(err).NotTo(BeNil())
		})
		It("should return a proper Subscription", func() {
			err := createSubscription(fakeOlmClientSet, testSubscription)
			Expect(err).To(BeNil())
			s, err := getSubscription(fakeOlmClientSet, "foosubscription", subscriptionTestNamespace)
			Expect(err).To(BeNil())
			Expect(s.Spec.Channel).To(Equal("foochannel"))
		})
	})

	Context("updateSubscription", func() {
		var currentSubscription *operatorv1alpha1.Subscription
		var targetSubscription *operatorv1alpha1.Subscription
		var fakeOlmClientSet *olmclient.Clientset
		BeforeEach(func() {
			currentSubscription = defaultTestSubscription.DeepCopy()
			targetSubscription = defaultTestSubscription.DeepCopy()
			targetSubscription.Spec.Channel = "updatedchannel"
			fakeOlmClientSet = olmclient.NewSimpleClientset()

		})
		It("should update a Subscription", func() {
			err := createSubscription(fakeOlmClientSet, currentSubscription)
			Expect(err).To(BeNil())
			err, changed := updateSubscription(fakeOlmClientSet, targetSubscription, currentSubscription)
			Expect(err).To(BeNil())
			Expect(changed).To(BeTrue())
		})
	})
})
