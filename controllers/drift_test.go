package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

const (
	mainReference          plumbing.ReferenceName = "refs/heads/main"
	originURL              string                 = "https://origin.url"
	targetURL              string                 = "https://target.url"
	hashCommitMainHead     string                 = "667679cce3942d3dec754b29d0f97500bba57978"
	hashCommitAmendedHead  string                 = "6ffb7b8f89075d66fba48c4d0000f8fb52720cf1"
	hashCommitTestBranch   string                 = "0e34ab1c94a4b588ddea45087e956b22bddfa8a2"
	hashCommitBugfixBranch string                 = "597db674d31dee964f464d84ee0b4f3797bb06dd"
	foo                    string                 = "foo"
	bar                    string                 = "bar"
	defaultNamespace       string                 = "default"
)

var (
	firstCommitReference = []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, mainReference),
		plumbing.NewHashReference(mainReference, plumbing.NewHash(hashCommitMainHead))}
	firstCommitAmendedReference = []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, mainReference),
		plumbing.NewHashReference(mainReference,
			plumbing.NewHash(hashCommitAmendedHead))}
	firstCommitReferenceWithMaster = []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.Master),
		plumbing.NewHashReference(plumbing.Master,
			plumbing.NewHash(hashCommitMainHead))}
	multipleCommitsReference = []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, mainReference),
		plumbing.NewHashReference(mainReference,
			plumbing.NewHash(hashCommitMainHead)),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("test"),
			plumbing.NewHash(hashCommitMainHead)),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("bugfix"),
			plumbing.NewHash(hashCommitMainHead)),
	}
	multipleCommitsWithDifferentHashReference = []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, mainReference),
		plumbing.NewHashReference(mainReference,
			plumbing.NewHash(hashCommitAmendedHead)),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("bugfix"),
			plumbing.NewHash(hashCommitBugfixBranch)),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("test"),
			plumbing.NewHash(hashCommitTestBranch)),
	}
	noHeadReference = []*plumbing.Reference{
		plumbing.NewHashReference(mainReference,
			plumbing.NewHash("first Commit"))}

	noCommits    = []*plumbing.Reference{plumbing.NewSymbolicReference(plumbing.HEAD, mainReference)}
	emptyCommits = []*plumbing.Reference{}
)
var _ = Describe("Git client", func() {

	var _ = Context("when interacting with Git", func() {
		var (
			mockGitClient                                  *MockClient
			mockRemoteClientOrigin, mockRemoteClientTarget *MockRemoteClient
			pattern                                        api.Pattern
		)

		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			mockGitClient = NewMockClient(ctrl)
			mockRemoteClientOrigin = NewMockRemoteClient(ctrl)
			mockRemoteClientTarget = NewMockRemoteClient(ctrl)
		})

		AfterEach(func() {
			e := k8sClient.Delete(context.Background(), &pattern)
			Expect(e).NotTo(HaveOccurred())
		})
		DescribeTable("when drifting", func(originRefs, targetRefs []*plumbing.Reference, originRef, targetRef string, expected bool, errOriginList, errTargetList, errOriginFilter, errTargetFilter error) {
			pattern = api.Pattern{
				ObjectMeta: v1.ObjectMeta{Name: foo, Namespace: defaultNamespace},
				TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
				Spec: api.PatternSpec{
					GitConfig: api.GitConfig{
						Hostname:       foo,
						PollInterval:   30,
						OriginRepo:     originURL,
						OriginRevision: originRef,
						TargetRepo:     targetURL,
						TargetRevision: targetRef}},
			}
			e := k8sClient.Create(context.Background(), &pattern)
			Expect(e).NotTo(HaveOccurred())

			remote := repositoryPair{
				name:      foo,
				namespace: defaultNamespace,
				gitClient: mockGitClient,
				kClient:   k8sClient,
			}
			mockGitClient.EXPECT().NewRemoteClient(&config.RemoteConfig{Name: "origin", URLs: []string{originURL}}).Times(1).Return(mockRemoteClientOrigin)
			mockGitClient.EXPECT().NewRemoteClient(&config.RemoteConfig{Name: "target", URLs: []string{targetURL}}).Times(1).Return(mockRemoteClientTarget)
			mockRemoteClientOrigin.EXPECT().List(&git.ListOptions{}).Times(1).Return(originRefs, errOriginList)
			if errOriginList == nil {
				mockRemoteClientTarget.EXPECT().List(&git.ListOptions{}).Times(1).Return(targetRefs, errTargetList)
			}

			hasDrifted, e := remote.hasDrifted()
			if e != nil {
				switch {
				case errOriginList != nil:
					Expect(e).To(Equal(errOriginList))
				case errTargetList != nil:
					Expect(e).To(Equal(errTargetList))
				case errOriginFilter != nil:
					Expect(e).To(Equal(errOriginFilter))
				case errTargetFilter != nil:
					Expect(e).To(Equal(errTargetFilter))
				}
				return
			}

			Expect(e).NotTo(HaveOccurred())
			Expect(hasDrifted).To(Equal(expected))
		},
			Entry("One commit with head main and same hash", firstCommitReference, firstCommitReference, "", "", false, nil, nil, nil, nil),
			Entry("One commit with head main and different hash", firstCommitReference, firstCommitAmendedReference, "", "", true, nil, nil, nil, nil),
			Entry("One commit with head main and head master and same hash", firstCommitReference, firstCommitReferenceWithMaster, "", "", false, nil, nil, nil, nil),
			Entry("Multiple commit with head main and branches with the same hash", multipleCommitsReference, multipleCommitsReference, "", "", false, nil, nil, nil, nil),
			Entry("Multiple commit with head main and branches with different hash", multipleCommitsReference, multipleCommitsWithDifferentHashReference, "", "", true, nil, nil, nil, nil),
			Entry("One commit with head main and target reference with the same hash", firstCommitReference, multipleCommitsReference, "", "test", false, nil, nil, nil, nil),
			Entry("One commit with origin reference and target reference with the same hash", firstCommitReference, multipleCommitsReference, "test", "test", false, nil, nil, nil, nil),
			// errors
			Entry("Error while retrieving the origin references", emptyCommits, nil, "", "", false, fmt.Errorf("no references found for origin %s", originURL), nil, nil, nil),
			Entry("Error while retrieving the target references", firstCommitReference, nil, "", "", false, nil, fmt.Errorf("error while retrieving target references %s", targetURL), nil, nil),
			Entry("One commit with no HEAD reference in origin", noHeadReference, noHeadReference, "", "", false, nil, nil, fmt.Errorf("unable to find HEAD for origin %s", originURL), nil),
			Entry("One commit with no HEAD reference in target", firstCommitReference, noHeadReference, "", "", false, nil, nil, nil, fmt.Errorf("unable to find HEAD for target %s", targetURL)),
			Entry("No commits found in origin", noCommits, noHeadReference, "", "", false, nil, nil, fmt.Errorf("unable to find HEAD for origin %s", originURL), nil),
			Entry("No commits found in target", firstCommitReference, noCommits, "", "", false, nil, nil, nil, fmt.Errorf("unable to find HEAD for target %s", targetURL)),
			Entry("Reference not found in origin", firstCommitAmendedReference, firstCommitReference, "reference/not/found", "", false, nil, nil, fmt.Errorf("unable to find refs/heads/reference/not/found for origin %s", originURL), nil),
			Entry("Reference not found in target", firstCommitAmendedReference, firstCommitReference, "", "reference/not/found", false, nil, nil, nil, fmt.Errorf("unable to find refs/heads/reference/not/found for target %s", targetURL)),
		)
	})
	var _ = Context("git reference", func() {

		DescribeTable("when retrieving the git reference", func(references []*plumbing.Reference, targetRef plumbing.ReferenceName, expected *plumbing.Reference) {
			ret := getReferenceByName(references, targetRef)
			if expected == nil {
				Expect(ret).To(BeNil())
				return
			}
			Expect(expected).To(Equal(ret))
		},
			Entry("When filtering for HEAD symbolic link and is found", firstCommitReference, plumbing.HEAD, plumbing.NewSymbolicReference(plumbing.HEAD, mainReference)),
			Entry("When filtering for ref/heads/main and is found", firstCommitReference, mainReference, plumbing.NewHashReference(mainReference, plumbing.NewHash(hashCommitMainHead))),

			// errors
			Entry("When the symbolic link for HEAD is not found", noHeadReference, plumbing.HEAD, nil),
			Entry("When the reference is not found", noCommits, mainReference, nil),
		)

	})
	var _ = Context("When interacting with the pair slice", func() {

		var (
			one   = &repositoryPair{name: "one", namespace: "default", nextCheck: time.Time{}.Add(time.Second)}
			three = &repositoryPair{name: "three", namespace: "default", nextCheck: time.Time{}.Add(3 * time.Second)}
			four  = &repositoryPair{name: "four", namespace: "default", nextCheck: time.Time{}.Add(4 * time.Second)}
			five  = &repositoryPair{name: "second", namespace: "default", nextCheck: time.Time{}.Add(5 * time.Second)}
		)
		It("sorts correctly the order", func() {
			watch := newWatcher(nil)
			watch.watch()
			By("adding four elements")
			watch.repoPairs = []*repositoryPair{five, three, one, four}
			sort.Sort(watch.repoPairs)
			Expect(watch.repoPairs).To(HaveLen(4))
			Expect(watch.repoPairs[0]).To(BeEquivalentTo(one))
			Expect(watch.repoPairs[1]).To(BeEquivalentTo(three))
			Expect(watch.repoPairs[2]).To(BeEquivalentTo(four))
			Expect(watch.repoPairs[3]).To(BeEquivalentTo(five))
			By("removing the first element")
			watch.repoPairs = watch.repoPairs[1:]
			sort.Sort(watch.repoPairs)
			Expect(watch.repoPairs[0]).To(BeEquivalentTo(three))
			Expect(watch.repoPairs[1]).To(BeEquivalentTo(four))
			Expect(watch.repoPairs[2]).To(BeEquivalentTo(five))
		})

	})

	var _ = Context("When updating the pattern conditions", func() {

		var (
			ctx     = context.Background()
			pattern api.Pattern
		)

		BeforeEach(func() {
			pattern = api.Pattern{
				ObjectMeta: v1.ObjectMeta{Name: foo, Namespace: defaultNamespace},
				TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
				Spec:       api.PatternSpec{GitConfig: api.GitConfig{Hostname: foo, PollInterval: 30}},
			}
			e := k8sClient.Create(ctx, &pattern)
			Expect(e).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			e := k8sClient.Delete(ctx, &pattern)
			Expect(e).NotTo(HaveOccurred())
		})
		It("adds the first condition", func() {
			var p api.Pattern
			timestamp := time.Time{}.Add(1 * time.Second)
			By("validating the pattern has no conditions yet")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.Status.Conditions).To(HaveLen(0))
			By("calling the update pattern conditions to add a new condition")
			e := updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, timestamp)
			Expect(e).NotTo(HaveOccurred())
			By("retrieving the pattern object once more and validating that it contains the new condition")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.Status.Conditions).To(HaveLen(1))
			Expect(p.Status.Conditions[0]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitInSync,
				Status:             v1core.ConditionTrue,
				LastUpdateTime:     v1.Time{Time: timestamp},
				LastTransitionTime: v1.Time{Time: timestamp},
				Message:            "Git repositories are in sync",
			}))
		})
		It("updates lastUpdate field when condition still occurs while condition is active", func() {
			var p api.Pattern
			firstTimestamp := time.Time{}.Add(1 * time.Second)
			By("calling the update pattern conditions to add the condition")
			e := updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, firstTimestamp)
			Expect(e).NotTo(HaveOccurred())
			By("calling the update pattern conditions again to trigger the update of the lastUpdate field")
			secondTimeStamp := time.Time{}.Add(2 * time.Second)
			e = updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, secondTimeStamp)
			Expect(e).NotTo(HaveOccurred())
			By("retrieving the pattern object")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.Status.Conditions).To(HaveLen(1))
			Expect(p.Status.Conditions[0]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitInSync,
				Status:             v1core.ConditionTrue,
				LastUpdateTime:     v1.Time{Time: secondTimeStamp},
				LastTransitionTime: v1.Time{Time: firstTimestamp},
				Message:            "Git repositories are in sync",
			}))
		})
		It("transitions to a new condition type as status true", func() {
			var p api.Pattern
			firstTimestamp := time.Time{}.Add(1 * time.Second)
			By("calling the update pattern conditions to add the condition")
			e := updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, firstTimestamp)
			Expect(e).NotTo(HaveOccurred())
			By("calling the update pattern conditions again to trigger the update of the lastUpdate field")
			secondTimeStamp := time.Time{}.Add(2 * time.Second)
			e = updatePatternConditions(k8sClient, api.GitOutOfSync, foo, defaultNamespace, secondTimeStamp)
			Expect(e).NotTo(HaveOccurred())
			By("retrieving the pattern object")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.Status.Conditions).To(HaveLen(2))
			Expect(p.Status.Conditions[0]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitInSync,
				Status:             v1core.ConditionFalse,
				LastUpdateTime:     v1.Time{Time: secondTimeStamp},
				LastTransitionTime: v1.Time{Time: firstTimestamp},
				Message:            "Git repositories are in sync",
			}))
			Expect(p.Status.Conditions[1]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitOutOfSync,
				Status:             v1core.ConditionTrue,
				LastUpdateTime:     v1.Time{Time: secondTimeStamp},
				LastTransitionTime: v1.Time{Time: secondTimeStamp},
				Message:            "Git repositories are out of sync",
			}))
		})
		It("transitions back to an existing condition type", func() {
			var p api.Pattern
			firstTimestamp := time.Time{}.Add(1 * time.Second)
			By("calling the update pattern conditions to add the condition")
			e := updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, firstTimestamp)
			Expect(e).NotTo(HaveOccurred())
			By("calling the update pattern conditions again to trigger the update of the lastUpdate field")
			secondTimeStamp := time.Time{}.Add(2 * time.Second)
			e = updatePatternConditions(k8sClient, api.GitOutOfSync, foo, defaultNamespace, secondTimeStamp)
			Expect(e).NotTo(HaveOccurred())
			thirdTimeStamp := time.Time{}.Add(3 * time.Second)
			e = updatePatternConditions(k8sClient, api.GitInSync, foo, defaultNamespace, thirdTimeStamp)
			Expect(e).NotTo(HaveOccurred())
			By("retrieving the pattern object")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &p)
			Expect(err).NotTo(HaveOccurred())
			Expect(p).NotTo(BeNil())
			Expect(p.Status.Conditions).To(HaveLen(2))
			Expect(p.Status.Conditions[0]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitInSync,
				Status:             v1core.ConditionTrue,
				LastUpdateTime:     v1.Time{Time: thirdTimeStamp},
				LastTransitionTime: v1.Time{Time: thirdTimeStamp},
				Message:            "Git repositories are in sync",
			}))
			Expect(p.Status.Conditions[1]).To(BeComparableTo(api.PatternCondition{
				Type:               api.GitOutOfSync,
				Status:             v1core.ConditionFalse,
				LastUpdateTime:     v1.Time{Time: thirdTimeStamp},
				LastTransitionTime: v1.Time{Time: secondTimeStamp},
				Message:            "Git repositories are out of sync",
			}))
		})
	})

})

var _ = Describe("Drift watcher", func() {
	var _ = Context("When watching for drifts", func() {
		var (
			patternFoo                         *api.Pattern
			ctrl                               *gomock.Controller
			mockGitClient                      *MockClient
			mockRemoteOrigin, mockRemoteTarget *MockRemoteClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			mockGitClient = NewMockClient(ctrl)
			mockRemoteOrigin = NewMockRemoteClient(ctrl)
			mockRemoteTarget = NewMockRemoteClient(ctrl)
			// Add the pattern in etcd
			patternFoo = &api.Pattern{
				ObjectMeta: v1.ObjectMeta{Name: foo, Namespace: defaultNamespace},
				TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
				Spec:       api.PatternSpec{GitConfig: api.GitConfig{OriginRepo: originURL, TargetRepo: targetURL}}}

			err := k8sClient.Create(context.TODO(), patternFoo)
			Expect(err).NotTo(HaveOccurred())

		})

		AfterEach(func() {

			err := k8sClient.Delete(context.TODO(), patternFoo)
			Expect(err).NotTo(HaveOccurred())
		})

		It("detects a drift between a pair of git repositories after the second check", func() {
			var (
				payloadDelivered bool
			)

			mockGitClient.EXPECT().NewRemoteClient(gomock.Any()).DoAndReturn(func(c *config.RemoteConfig) RemoteClient {
				if c.Name == "origin" {
					return mockRemoteOrigin
				}
				return mockRemoteTarget
			}).AnyTimes()

			mockRemoteOrigin.EXPECT().List(gomock.Any()).Return(firstCommitReference, nil).AnyTimes()
			mockRemoteTarget.EXPECT().List(gomock.Any()).DoAndReturn(func(_ *git.ListOptions) ([]*plumbing.Reference, error) {
				if !payloadDelivered {
					payloadDelivered = true
					return firstCommitReference, nil
				}
				return multipleCommitsWithDifferentHashReference, nil
			}).AnyTimes()
			watch, closeCh := newDriftWatcher(k8sClient, logr.New(log.NullLogSink{}), mockGitClient)

			// Add the pair
			timestamp := time.Now()
			err := watch.add(foo, defaultNamespace, 1)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, patternFoo)
				Expect(err).NotTo(HaveOccurred())
				return len(patternFoo.Status.Conditions) == 1
			}).WithPolling(time.Second).WithTimeout(10*time.Second).Should(BeTrue(), "expected number of conditions %d but found %d", 1, len(patternFoo.Status.Conditions))
			// check that the conditions reflect the drift polling
			Expect(patternFoo.Status.Conditions[0].Type).To(Equal(api.GitInSync))
			Expect(patternFoo.Status.Conditions[0].Status).To(Equal(v1core.ConditionTrue))
			Expect(patternFoo.Status.Conditions[0].LastUpdateTime.Time).To(BeTemporally(">", timestamp))
			Expect(patternFoo.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally(">", timestamp))
			// wait for the second check to report the drift
			Eventually(func() bool {
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, patternFoo)
				Expect(err).NotTo(HaveOccurred())
				return len(patternFoo.Status.Conditions) == 2
			}).WithPolling(time.Second).WithTimeout(10*time.Second).Should(BeTrue(), "expected number of conditions %d but found %d", 2, len(patternFoo.Status.Conditions))
			// notify the routine that we're closing so that it doesn't keep checking for more drifts
			close(closeCh)
			// retrieve the first element in the slice
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, patternFoo)
			Expect(err).NotTo(HaveOccurred())

			// previous condition should have status false
			Expect(patternFoo.Status.Conditions[0].Type).To(Equal(api.GitInSync))
			Expect(patternFoo.Status.Conditions[0].Status).To(Equal(v1core.ConditionFalse))
			Expect(patternFoo.Status.Conditions[0].LastUpdateTime.Time).To(BeTemporally("==", patternFoo.Status.Conditions[1].LastUpdateTime.Time))
			Expect(patternFoo.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally("==", patternFoo.Status.Conditions[0].LastUpdateTime.Time.Add(-1*time.Second)))
			// new condition should show the repositories have drifted
			Expect(patternFoo.Status.Conditions[1].Type).To(Equal(api.GitOutOfSync))
			Expect(patternFoo.Status.Conditions[1].Status).To(Equal(v1core.ConditionTrue))
			Expect(patternFoo.Status.Conditions[1].LastTransitionTime.Time).To(BeTemporally("==", patternFoo.Status.Conditions[1].LastUpdateTime.Time))
			Expect(patternFoo.Status.Conditions[1].LastUpdateTime.Time).To(BeTemporally("==", patternFoo.Status.Conditions[0].LastTransitionTime.Time.Add(time.Second)))
		})

	})
	var _ = Context("when evaluating the processing order", func() {
		var (
			mockGitClient          *MockClient
			mockRemote             *MockRemoteClient
			patternBar, patternFoo *api.Pattern
			ctrl                   *gomock.Controller
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockGitClient = NewMockClient(ctrl)
			mockRemote = NewMockRemoteClient(ctrl)

			patternFoo = &api.Pattern{
				ObjectMeta: v1.ObjectMeta{Name: foo, Namespace: defaultNamespace},
				TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
				Spec:       api.PatternSpec{GitConfig: api.GitConfig{OriginRepo: originURL, TargetRepo: targetURL}}}
			patternBar = &api.Pattern{
				ObjectMeta: v1.ObjectMeta{Name: bar, Namespace: defaultNamespace},
				TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
				Spec:       api.PatternSpec{GitConfig: api.GitConfig{OriginRepo: originURL, TargetRepo: targetURL}}}
			e := k8sClient.Create(context.Background(), patternFoo)
			Expect(e).NotTo(HaveOccurred())
			e = k8sClient.Create(context.Background(), patternBar)
			Expect(e).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.TODO(), patternFoo)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), patternBar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("processes two pairs of git repositories in order of shortest interval", func() {
			mockGitClient.EXPECT().NewRemoteClient(gomock.Any()).Return(mockRemote).AnyTimes()
			mockRemote.EXPECT().List(gomock.Any()).Return(firstCommitReference, nil).AnyTimes()

			watch := newWatcher(mockGitClient)
			watch.watch()

			// Add both reference pairs and wait for the drift evaluation to kick in and add the first condition
			err := watch.add(foo, defaultNamespace, 5)
			Expect(err).NotTo(HaveOccurred())
			err = watch.add(bar, defaultNamespace, 1)
			Expect(err).NotTo(HaveOccurred())
			// check the order of processing pairs
			Expect(watch.repoPairs[0].name).To(Equal(bar))
			Expect(watch.repoPairs[1].name).To(Equal(foo))
			Eventually(func() bool {
				var pFoo, pBar api.Pattern
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &pFoo)
				Expect(err).NotTo(HaveOccurred())
				err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: bar, Namespace: defaultNamespace}, &pBar)
				Expect(err).NotTo(HaveOccurred())
				return len(pFoo.Status.Conditions) == 0 && len(pBar.Status.Conditions) == 1
			}).WithPolling(time.Second).WithTimeout(10*time.Second).Should(BeTrue(),
				"expected number of conditions for foo %d and bar %d but found %d and %d respectively ", 0, len(patternFoo.Status.Conditions), 1, len(patternBar.Status.Conditions))
			// Confirm the status contains a new condition with type git in sync
			var pattern api.Pattern
			err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: bar, Namespace: defaultNamespace}, &pattern)
			Expect(err).NotTo(HaveOccurred())
			// check that the conditions reflect the drift polling
			Expect(pattern.Status.Conditions[0].Type).To(Equal(api.GitInSync))
			Expect(pattern.Status.Conditions[0].Status).To(Equal(v1core.ConditionTrue))
		})
		It("removes the fist pair and adds it again with longer interval to ensure it is requeued the last", func() {
			mockGitClient.EXPECT().NewRemoteClient(gomock.Any()).Return(mockRemote).AnyTimes()
			mockRemote.EXPECT().List(gomock.Any()).Return(firstCommitReference, nil).AnyTimes()

			watch := newWatcher(mockGitClient)
			watch.watch()
			// Add both reference pairs and wait for the drift evaluation to kick in and add the first condition
			err := watch.add(foo, defaultNamespace, 5)
			Expect(err).NotTo(HaveOccurred())
			err = watch.add(bar, defaultNamespace, 4)
			Expect(err).NotTo(HaveOccurred())
			// remove the first element
			err = watch.remove(bar, defaultNamespace)
			Expect(err).NotTo(HaveOccurred())
			// readd the first element but with longer interval
			err = watch.add(bar, defaultNamespace, 5)
			Expect(err).NotTo(HaveOccurred())
			// check the order of processing pairs
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[1].name).To(Equal(bar))
			// wait for the first element to be processed at least once
			var pattern api.Pattern
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &pattern)
				Expect(err).NotTo(HaveOccurred())
				return len(pattern.Status.Conditions) == 1
			}).WithPolling(time.Second).WithTimeout(10*time.Second).Should(BeTrue(), "expected number of conditions to be %d but found %d", 1, len(pattern.Status.Conditions))
		})

		It("updates the interval of an existing repository pair", func() {
			mockGitClient.EXPECT().NewRemoteClient(gomock.Any()).Return(mockRemote).AnyTimes()
			mockRemote.EXPECT().List(gomock.Any()).Return(firstCommitReference, nil).AnyTimes()

			watch := newWatcher(mockGitClient)
			watch.watch()
			// Add both reference pairs and wait for the drift evaluation to kick in and add the first condition
			err := watch.add(foo, defaultNamespace, 5)
			Expect(err).NotTo(HaveOccurred())
			err = watch.add(bar, defaultNamespace, 4)
			Expect(err).NotTo(HaveOccurred())
			// update the first element but with longer interval
			err = watch.updateInterval(bar, defaultNamespace, 6)
			Expect(err).NotTo(HaveOccurred())
			// check the order of processing pairs
			Expect(watch.repoPairs[0].name).To(Equal(foo))
			Expect(watch.repoPairs[1].name).To(Equal(bar))
			// wait for the first element to be processed at least once
			var pattern api.Pattern
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: foo, Namespace: defaultNamespace}, &pattern)
				Expect(err).NotTo(HaveOccurred())
				return len(pattern.Status.Conditions) == 1
			}).WithPolling(time.Second).WithTimeout(10*time.Second).Should(BeTrue(), "expected number of conditions to be %d but found %d", 1, len(pattern.Status.Conditions))
		})
	})

	var _ = Context("when running in parallel", func() {
		const (
			defaultNamespace = "default"
		)
		var (
			mockGitClient *MockClient
			mockRemote    *MockRemoteClient
			ctrl          *gomock.Controller
		)
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockGitClient = NewMockClient(ctrl)
			mockRemote = NewMockRemoteClient(ctrl)
			// add references
			for i := 0; i < 1000; i++ {
				p := &api.Pattern{
					ObjectMeta: v1.ObjectMeta{Name: fmt.Sprintf("load-%d", i), Namespace: defaultNamespace},
					TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
					Spec:       api.PatternSpec{GitConfig: api.GitConfig{OriginRepo: originURL, TargetRepo: targetURL}}}
				e := k8sClient.Create(context.Background(), p)
				Expect(e).NotTo(HaveOccurred())
			}

		})

		AfterEach(func() {
			// add references
			for i := 0; i < 1000; i++ {
				p := &api.Pattern{
					ObjectMeta: v1.ObjectMeta{Name: fmt.Sprintf("load-%d", i), Namespace: defaultNamespace},
					TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()}}
				e := k8sClient.Delete(context.Background(), p)
				Expect(e).NotTo(HaveOccurred())
			}
		})
		It("adds,removes and check for existing pairs in parallel load with random intervals", func() {
			mockGitClient.EXPECT().NewRemoteClient(gomock.Any()).Return(mockRemote).AnyTimes()
			mockRemote.EXPECT().List(gomock.Any()).Return(firstCommitReference, nil).AnyTimes()

			watch, _ := newDriftWatcher(k8sClient, logr.New(log.NullLogSink{}), mockGitClient)
			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				for i := 0; i < 1000; i++ {
					// set interval between 1-2 seconds to force the trigger of the timer function during the test
					interval := rand.Intn(2) + 1                    //nolint:gosec
					name := fmt.Sprintf("load-%d", rand.Intn(1000)) //nolint:gosec
					for watch.isWatching(name, defaultNamespace) {
						name = fmt.Sprintf("load-%d", rand.Intn(1000)) //nolint:gosec
					}
					Expect(watch.add(name, defaultNamespace, interval)).NotTo(HaveOccurred())
				}
				wg.Done()
			}()
			go func() {
				var deleted int
				for deleted < 1000 {
					name := fmt.Sprintf("load-%d", rand.Intn(1000)) //nolint:gosec
					if watch.isWatching(name, defaultNamespace) {
						Expect(watch.remove(name, defaultNamespace)).NotTo(HaveOccurred())
						deleted++
					}
				}
				wg.Done()
			}()
			wg.Wait()
		})
	})
})

func newWatcher(gitClient GitClient) *watcher {
	return &watcher{
		kClient:   k8sClient,
		repoPairs: repositoryPairs{},
		endCh:     make(chan any),
		mutex:     &sync.Mutex{},
		gitClient: gitClient,
		logger:    logr.New(log.NullLogSink{}),
	}
}
