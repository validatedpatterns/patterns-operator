package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("gitops config controller", Ordered, func() {

	var (
		defaultConfiguration = api.GitOpsConfig{
			ObjectMeta: v1.ObjectMeta{
				Name:      gitOpsConfigDefaultNamespacedName.Name,
				Namespace: gitOpsConfigDefaultNamespacedName.Namespace},
			Spec: api.GitOpsConfigSpec{
				OperatorChannel: "stable",
				OperatorSource:  "redhat-operators",
				OperatorCSV:     "v1.4.0",
			},
		}
	)

	BeforeAll(func() {
		err := k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: gitOpsConfigDefaultNamespacedName.Namespace}})
		Expect(err).NotTo(HaveOccurred())
	})
	var _ = Context("default configuration", func() {
		DescribeTable("aplies the default configuration", func(defined, expected api.GitOpsConfigSpec) {
			r := GitOpsConfigReconciler{}
			c := api.GitOpsConfig{
				Spec: api.GitOpsConfigSpec{
					OperatorChannel: defined.OperatorChannel,
					OperatorSource:  defined.OperatorSource,
					OperatorCSV:     defined.OperatorCSV}}
			res := r.applyGitOpsConfigDefaults(&c)
			Expect(res.Spec).To(BeEquivalentTo(expected))
		},
			Entry("against a spec with default values", api.GitOpsConfigSpec{}, defaultConfiguration.Spec),
			Entry("against a spec with only channel defined",
				api.GitOpsConfigSpec{
					OperatorChannel: "test"},
				api.GitOpsConfigSpec{
					OperatorChannel: "test",
					OperatorSource:  defaultConfiguration.Spec.OperatorSource,
					OperatorCSV:     defaultConfiguration.Spec.OperatorCSV,
				}),
			Entry("against a spec with only operator source defined",
				api.GitOpsConfigSpec{
					OperatorSource: "source"},
				api.GitOpsConfigSpec{
					OperatorChannel: defaultConfiguration.Spec.OperatorChannel,
					OperatorSource:  "source",
					OperatorCSV:     defaultConfiguration.Spec.OperatorCSV,
				}),
			Entry("against a spec with only the operator CSV defined",
				api.GitOpsConfigSpec{
					OperatorCSV: "v1.0.0"},
				api.GitOpsConfigSpec{
					OperatorChannel: defaultConfiguration.Spec.OperatorChannel,
					OperatorSource:  defaultConfiguration.Spec.OperatorSource,
					OperatorCSV:     "v1.0.0",
				}),
		)
	},
	)

	var _ = Context("finalize object", func() {
		It("returns no error when there are no pattern objects stored in etcd", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			c := api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Finalizers: []string{api.GitOpsConfigFinalizer},
				},
			}
			err := r.finalizeObject(&c)
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns an error when there are still patterns stored in etcd", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			c := api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Finalizers: []string{api.GitOpsConfigFinalizer},
					Name:       "foo",
					Namespace:  "bar",
				},
			}
			p := api.Pattern{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "default"}}
			By("adding a pattern")
			err := k8sClient.Create(context.TODO(), &p)
			Expect(err).NotTo(HaveOccurred())
			err = r.finalizeObject(&c)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeEquivalentTo(fmt.Errorf("unable to remove the GitOpsConfig %s in %s as not all pattern resources have been removed,", c.Name, c.Namespace)))
			err = k8sClient.Delete(context.TODO(), &p)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns without error when the object has no finalizer", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			c := api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			err := r.finalizeObject(&c)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	var _ = Context("reconciliation with error and requeuing", func() {

		AfterEach(func() {
			list := api.GitOpsConfigList{}
			err := k8sClient.List(context.TODO(), &list)
			Expect(err).NotTo(HaveOccurred())
			for _, item := range list.Items {
				Expect(k8sClient.Delete(context.Background(), &item))
			}
		})

		var minute = time.Minute
		It("reconciles with delay", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "default"}}
			Expect(k8sClient.Create(context.Background(), &g)).NotTo(HaveOccurred())
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), &minute)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: minute}))
		})

		It("reconciles with delay due to unexpected error updating the object", func() {
			r := GitOpsConfigReconciler{Client: k8sClient, logger: logr.New(log.NullLogSink{})}
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "default"}}
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), &minute)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: minute}))

		})

		It("reconciles without delay as no duration value was given", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "default"}}
			Expect(k8sClient.Create(context.Background(), &g)).NotTo(HaveOccurred())
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), nil)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
		})
	})

	var _ = Context("reconciliation loop", func() {

		var (
			baseConfig  api.GitOpsConfig
			nsOperators = corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: gitOpsConfigDefaultNamespacedName.Namespace}}
			nsGitOps    = corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: applicationNamespace}}
		)
		BeforeEach(func() {
			baseConfig = api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      gitOpsConfigDefaultNamespacedName.Name,
					Namespace: gitOpsConfigDefaultNamespacedName.Namespace},
				Spec: api.GitOpsConfigSpec{
					OperatorChannel: "stable",
					OperatorSource:  "redhat-operators",
					OperatorCSV:     "v1.4.0",
				},
			}

		})
		It("ignores the request that doesn't match the name and namespace criteria", func() {
			reconciler := newFakeGitOpsConfigReconciler(&baseConfig, &nsOperators)
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "bar",
					Name:      "foo",
				},
			}
			res, err := reconciler.Reconcile(context.TODO(), request)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
			obj := api.GitOpsConfig{}
			err = reconciler.Client.Get(context.Background(), types.NamespacedName{
				Namespace: "bar",
				Name:      "foo",
			}, &obj)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
		It("processes a request that matches the name and namespace criteria but lacks the finalizers and has not been marked for deletion", func() {

		})
		It("removes the finalizer when there are no more patterns in the system and the deletion time has been set", func() {
			reconciler := newFakeGitOpsConfigReconciler(&nsOperators)
			By("adding the gitopsconfig instance with a finalizer and a deletion time")
			baseConfig.ObjectMeta.Finalizers = []string{api.GitOpsConfigFinalizer}
			baseConfig.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
			Expect(reconciler.Client.Create(context.Background(), &baseConfig)).NotTo(HaveOccurred())

			By("reconciling")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
			g := api.GitOpsConfig{}
			err = reconciler.Client.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("removes the object when it has no finalizers", func() {
			reconciler := newFakeGitOpsConfigReconciler(&nsOperators)
			By("adding the gitopsconfig instance with a finalizer and a deletion time")
			baseConfig.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
			Expect(reconciler.Client.Create(context.Background(), &baseConfig)).NotTo(HaveOccurred())

			By("reconciling")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
			g := api.GitOpsConfig{}
			err = k8sClient.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("completes the positive path in the reconciliation loop from start to end", func() {
			reconciler := newFakeGitOpsConfigReconciler(&baseConfig, &nsOperators)
			By("reconciling the first time to add the finalizer")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			_, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			g := api.GitOpsConfig{}
			err = reconciler.Client.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(err).NotTo(HaveOccurred())
			Expect(g.Status.LastError).To(BeEmpty())
			Expect(g.ObjectMeta.Finalizers).To(ContainElement(api.GitOpsConfigFinalizer))
			By("reconciling the second time to create the subscription for the openshift-gitops-operator")
			req = ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: time.Minute}))
			subs, err := reconciler.olmClient.OperatorsV1alpha1().Subscriptions("openshift-operators").Get(context.Background(), "openshift-gitops-operator", v1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(subs.Spec).To(BeEquivalentTo(newSubscription(&defaultConfiguration).Spec))
			By("reconciling for the third time where the subscription has been created but the gitops operator has not yet been deployed")
			res, err = reconciler.Reconcile(context.Background(), req)
			Expect(err).To(BeEquivalentTo(fmt.Errorf("waiting for creation")))
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: time.Minute}))
			By("reconciling for the last time once the gitops operator is being deployed or has completed deployment (determined by the existence of the openshift-gitops namespace)")
			Expect(reconciler.Client.Create(context.Background(), &nsGitOps)).NotTo(HaveOccurred())
			res, err = reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
		})
	})
})

func newFakeGitOpsConfigReconciler(initObjects ...runtime.Object) *GitOpsConfigReconciler {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(initObjects...).Build()

	return &GitOpsConfigReconciler{
		Scheme:    scheme.Scheme,
		Client:    fakeClient,
		olmClient: olmclient.NewSimpleClientset(),
	}
}
