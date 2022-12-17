package controllers

import (
	"context"
	"fmt"
	"time"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("gitops config controller", Ordered, func() {

	var (
		defaultConfiguration = api.GitOpsConfig{
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
		DescribeTable("aplies the default configuration", func(channel, source, csv string) {
			r := GitOpsConfigReconciler{}
			c := api.GitOpsConfig{
				Spec: api.GitOpsConfigSpec{
					OperatorChannel: channel,
					OperatorSource:  source,
					OperatorCSV:     csv}}
			res := r.applyGitOpsConfigDefaults(&c)
			Expect(res).To(Equal(defaultConfiguration))
		},
			Entry("against a spec with default values", "", "", ""),
			Entry("against a spec with only channel defined", "test", "", ""),
			Entry("against a spec with only operator source defined", "", "source", ""),
			Entry("against a spec with only the operator CSV defined", "", "", "v1.0.0"))
	})

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
			By("adding a pattern")
			err := k8sClient.Create(context.TODO(), &api.Pattern{})
			Expect(err).NotTo(HaveOccurred())
			err = r.finalizeObject(&c)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeEquivalentTo(fmt.Errorf("unable to remove the GitOpsConfig %s in %s as not all pattern resources have been removed,", c.Name, c.Namespace)))
		})

		It("returns without error when the object has no finalizer", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			c := api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			By("adding a pattern")
			err := k8sClient.Create(context.TODO(), &api.Pattern{})
			Expect(err).NotTo(HaveOccurred())
			err = r.finalizeObject(&c)
			Expect(err).To(HaveOccurred())
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
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "bar"}}
			Expect(k8sClient.Create(context.Background(), &g)).NotTo(HaveOccurred())
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), &minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: minute}))
		})

		It("reconciles with delay due to unexpected error updating the object", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "bar"}}
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), &minute)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: minute}))

		})

		It("reconciles without delay as no duration value was given", func() {
			r := GitOpsConfigReconciler{Client: k8sClient}
			g := api.GitOpsConfig{ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "bar"}}
			Expect(k8sClient.Create(context.Background(), &g)).NotTo(HaveOccurred())
			res, err := r.onReconcileErrorWithRequeue(&g, "reason", fmt.Errorf("an error has ocurred"), nil)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{}))
		})
	})

	var _ = Context("reconciliation loop", func() {

		var (
			baseConfig api.GitOpsConfig
		)
		BeforeEach(func() {
			baseConfig = api.GitOpsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      gitOpsConfigDefaultNamespacedName.Name,
					Namespace: gitOpsConfigDefaultNamespacedName.Namespace}}

		})
		It("ignores the request that doesn't match the name and namespace criteria", func() {
			reconciler := newFakeReconciler()
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
		FIt("processes a request that matches the name and namespace criteria but lacks the finalizers and has not been marked for deletion", func() {
			reconciler := newFakeReconciler(&baseConfig)

			By("reconciling")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: time.Minute, Requeue: true}))
			g := api.GitOpsConfig{}
			err = k8sClient.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(err).NotTo(HaveOccurred())
			Expect(g.Status.LastError).To(BeEmpty())
			Expect(g.ObjectMeta.Finalizers).To(ContainElement(api.GitOpsConfigFinalizer))
		})
		It("removes the finalizer when there are no more patterns in the system and the deletion time has been set", func() {
			reconciler := GitOpsConfigReconciler{
				Client:     k8sClient,
				olmClient:  olmclient.NewSimpleClientset(),
				fullClient: kubernetes.NewForConfigOrDie(testEnv.Config),
			}
			By("adding the gitopsconfig instance with a finalizer and a deletion time")
			baseConfig.ObjectMeta.Finalizers = []string{api.GitOpsConfigFinalizer}
			baseConfig.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
			Expect(k8sClient.Create(context.Background(), &baseConfig)).NotTo(HaveOccurred())

			By("reconciling")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: time.Minute}))
			g := api.GitOpsConfig{}
			err = k8sClient.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(err).NotTo(HaveOccurred())
			Expect(g.Status.LastError).To(BeEmpty())
			Expect(g.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("removes the object when it has no finalizers", func() {
			reconciler := GitOpsConfigReconciler{
				Client:     k8sClient,
				olmClient:  olmclient.NewSimpleClientset(),
				fullClient: kubernetes.NewForConfigOrDie(testEnv.Config),
			}
			By("adding the gitopsconfig instance with a finalizer and a deletion time")
			baseConfig.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
			Expect(k8sClient.Create(context.Background(), &baseConfig)).NotTo(HaveOccurred())

			By("reconciling")
			req := ctrl.Request{NamespacedName: gitOpsConfigDefaultNamespacedName}
			res, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeEquivalentTo(reconcile.Result{RequeueAfter: time.Minute}))
			g := api.GitOpsConfig{}
			err = k8sClient.Get(context.Background(), gitOpsConfigDefaultNamespacedName, &g)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func newFakeReconciler(initObjects ...runtime.Object) *GitOpsConfigReconciler {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(initObjects...).Build()
	return &GitOpsConfigReconciler{
		Client:    fakeClient,
		olmClient: olmclient.NewSimpleClientset(),
	}
}
