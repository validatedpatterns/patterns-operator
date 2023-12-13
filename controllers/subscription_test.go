package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes/fake"
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
			Expect(err).ToNot(HaveOccurred())
			changed, err := updateSubscription(fakeOlmClientSet, targetSubscription, currentSubscription)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
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
