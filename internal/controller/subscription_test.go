package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

var defaultTestSubscription = operatorv1alpha1.Subscription{
	Spec: &operatorv1alpha1.SubscriptionSpec{
		CatalogSource:          "foosource",
		CatalogSourceNamespace: "foosourcenamespace",
		Package:                "foooperator",
		Channel:                "foochannel",
		InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
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
		Namespace: OperatorNamespace,
	},
}

var defaultTestSubConfigMap = corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      OperatorConfigMap,
		Namespace: OperatorNamespace,
	},
	Data: map[string]string{
		"gitops.installApprovalPlan": "Manual",
		"gitops.catalogSource":       "foo-source",
		"gitops.sourceNamespace":     "foo-source-namespace",
		"gitops.name":                "foo-name",
		"gitops.channel":             "foo-channel",
		"gitops.csv":                 "1.2.3",
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
			Expect(err).ToNot(HaveOccurred())
			_, err = getSubscription(fakeOlmClientSet, "foo")
			Expect(err).To(HaveOccurred())
		})

		It("should return a proper Subscription", func() {
			err := createSubscription(fakeOlmClientSet, testSubscription)
			Expect(err).ToNot(HaveOccurred())
			sub, err := getSubscription(fakeOlmClientSet, "foosubscription")
			Expect(err).ToNot(HaveOccurred())
			Expect(sub.Spec.Channel).To(Equal("foochannel"))
			Expect(sub.Spec.CatalogSource).To(Equal("foosource"))
			Expect(sub.Spec.CatalogSourceNamespace).To(Equal("foosourcenamespace"))
			Expect(sub.Spec.Package).To(Equal("foooperator"))
			Expect(sub.Spec.Channel).To(Equal("foochannel"))
			Expect(sub.Spec.StartingCSV).To(BeEmpty())
			Expect(sub.Spec.InstallPlanApproval).To(Equal(operatorv1alpha1.ApprovalAutomatic))
		})
	})

	Context("newSubscriptionFromConfigMap", func() {
		var testConfigMap *corev1.ConfigMap
		var fakeClientSet *kubeclient.Clientset

		BeforeEach(func() {
			fakeClientSet = kubeclient.NewSimpleClientset()
			testConfigMap = defaultTestSubConfigMap.DeepCopy()
		})

		It("should handle the absence of the ConfigMap gracefully", func() {
			sub, err := newSubscriptionFromConfigMap(fakeClientSet)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).NotTo(BeNil())
			Expect(sub.Spec.CatalogSource).To(Equal(GitOpsDefaultCatalogSource))
			Expect(sub.Spec.CatalogSourceNamespace).To(Equal(GitOpsDefaultCatalogSourceNamespace))
			Expect(sub.Spec.Package).To(Equal(GitOpsDefaultPackageName))
			Expect(sub.Spec.Channel).To(Equal(GitOpsDefaultChannel))
			Expect(sub.Spec.StartingCSV).To(BeEmpty())
			Expect(sub.Spec.InstallPlanApproval).To(Equal(operatorv1alpha1.ApprovalAutomatic))
		})

		It("should create a Subscription from a configmap", func() {
			_, err := fakeClientSet.CoreV1().ConfigMaps(OperatorNamespace).Create(context.Background(), testConfigMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			sub, err := newSubscriptionFromConfigMap(fakeClientSet)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).NotTo(BeNil())
			Expect(sub.Spec.CatalogSource).To(Equal("foo-source"))
			Expect(sub.Spec.CatalogSourceNamespace).To(Equal("foo-source-namespace"))
			Expect(sub.Spec.Package).To(Equal("foo-name"))
			Expect(sub.Spec.Channel).To(Equal("foo-channel"))
			Expect(sub.Spec.StartingCSV).To(Equal("1.2.3"))
			Expect(sub.Spec.InstallPlanApproval).To(Equal(operatorv1alpha1.ApprovalManual))
		})
	})
})

var _ = Describe("UpdateSubscription", func() {
	var (
		client         *olmclient.Clientset
		target         *operatorv1alpha1.Subscription
		current        *operatorv1alpha1.Subscription
		subscriptionNs string
	)

	BeforeEach(func() {
		client = olmclient.NewSimpleClientset()
		subscriptionNs = "openshift-operators"

		current = &operatorv1alpha1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-subscription",
				Namespace: subscriptionNs,
			},
			Spec: &operatorv1alpha1.SubscriptionSpec{
				CatalogSourceNamespace: "default",
				CatalogSource:          "test-catalog",
				Channel:                "stable",
				Package:                "test-package",
				InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
				StartingCSV:            "v1.0.0",
			},
		}
		target = current.DeepCopy()
	})

	Context("when current subscription is nil", func() {
		It("should return an error", func() {
			changed, err := updateSubscription(client, target, nil)
			Expect(changed).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("current subscription was nil"))
		})
	})

	Context("when target subscription is nil", func() {
		It("should return an error", func() {
			changed, err := updateSubscription(client, nil, current)
			Expect(changed).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("target subscription was nil"))
		})
	})

	Context("when the subscription specs are the same", func() {
		It("should return false and no error", func() {
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when the subscription specs are different", func() {
		BeforeEach(func() {
			_, _ = client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Create(context.Background(), current, metav1.CreateOptions{})
		})

		It("channel difference should return true and update the current subscription", func() {
			target.Spec.Channel = "beta"
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.Channel).To(Equal("beta"))
		})

		It("catalgsource difference should return true and update the current subscription", func() {
			target.Spec.CatalogSource = "somesource"
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.CatalogSource).To(Equal("somesource"))
		})

		It("catalogsourcenamespace difference should return true and update the current subscription", func() {
			target.Spec.CatalogSourceNamespace = "another"
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.CatalogSourceNamespace).To(Equal("another"))
		})

		It("package difference should return true and update the current subscription", func() {
			target.Spec.Package = "notdefault"
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.Package).To(Equal("notdefault"))
		})

		It("InstallPlanApproval difference should return true and update the current subscription", func() {
			target.Spec.InstallPlanApproval = operatorv1alpha1.ApprovalManual
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.InstallPlanApproval).To(Equal(operatorv1alpha1.ApprovalManual))
		})

		It("StartingCSV difference should return true and update the current subscription", func() {
			target.Spec.StartingCSV = "v1.1.0"
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.StartingCSV).To(Equal("v1.1.0"))
		})

		It("Config.Env difference should return true and update the current subscription", func() {
			tmp := &operatorv1alpha1.SubscriptionConfig{
				Env: []corev1.EnvVar{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
			}
			target.Spec.Config = tmp
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			updated, err := client.OperatorsV1alpha1().Subscriptions(subscriptionNs).Get(context.Background(), current.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Spec.Config.Env[0].Name).To(Equal("foo"))
		})
	})

	Context("when there is an error updating the subscription", func() {
		BeforeEach(func() {
			client.PrependReactor("update", "subscriptions", func(testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("update error")
			})
			target.Spec.Channel = "beta"
		})

		It("should return true and an error", func() {
			changed, err := updateSubscription(client, target, current)
			Expect(changed).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("update error"))
		})
	})
})
