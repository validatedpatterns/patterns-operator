package controllers

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func prefixArray(a []string, prefix string) []string {
	b := []string{}
	for _, i := range a {
		b = append(b, fmt.Sprintf("%s%s", prefix, i))
	}
	return b
}

var _ = Describe("Argo Pattern", func() {
	var pattern *api.Pattern
	var defaultValueFiles []string
	BeforeEach(func() {
		pattern = &api.Pattern{
			ObjectMeta: v1.ObjectMeta{Name: "multicloud-gitops-test", Namespace: defaultNamespace},
			TypeMeta:   v1.TypeMeta{Kind: "Pattern", APIVersion: api.GroupVersion.String()},
			Spec: api.PatternSpec{
				ClusterGroupName: "foogroup",
			},
			Status: api.PatternStatus{
				ClusterPlatform: "AWS",
				ClusterVersion:  "4.12",
				ClusterName:     "bar",
			},
		}
		defaultValueFiles = []string{
			"/values-global.yaml",
			"/values-foogroup.yaml",
			"/values-AWS.yaml",
			"/values-AWS-4.12.yaml",
			"/values-AWS-foogroup.yaml",
			"/values-4.12-foogroup.yaml",
			"/values-bar.yaml",
		}
	})

	Describe("Testing applicationName function", func() {
		Context("Default", func() {
			It("Returns default application name", func() {
				Expect(applicationName(*pattern)).To(Equal("multicloud-gitops-test-foogroup"))
			})
		})
	})

	Describe("Testing newApplicationValueFiles function", func() {
		Context("Default", func() {
			It("Returns a default set of values", func() {
				valueFiles := newApplicationValueFiles(*pattern, "")
				Expect(valueFiles).To(Equal(defaultValueFiles))
			})
			It("Returns a default set of values with prefix", func() {
				valueFiles := newApplicationValueFiles(*pattern, "myprefix")
				Expect(valueFiles).To(Equal(prefixArray(defaultValueFiles, "myprefix")))
			})
		})

		Context("With extra valuefiles", func() {
			BeforeEach(func() {
				pattern.Spec.ExtraValueFiles = []string{
					"test1.yaml",
					"test2.yaml",
				}
			})
			It("Returns a default set of values and extravaluefiles without prefix", func() {
				valueFiles := newApplicationValueFiles(*pattern, "")
				Expect(valueFiles).To(Equal(append(defaultValueFiles,
					"/test1.yaml",
					"/test2.yaml")))
			})
			It("Returns a default set of values and extravaluefiles with prefix", func() {
				valueFiles := newApplicationValueFiles(*pattern, "myprefix")
				Expect(valueFiles).To(Equal(append(prefixArray(defaultValueFiles, "myprefix"),
					"myprefix/test1.yaml",
					"myprefix/test2.yaml")))
			})
		})
		Context("With extra valuefiles with leading slashes", func() {
			BeforeEach(func() {
				pattern.Spec.ExtraValueFiles = []string{
					"/test1.yaml",
					"/test2.yaml",
				}
			})
			It("Returns a default set of values and extravaluefiles", func() {
				valueFiles := newApplicationValueFiles(*pattern, "")
				Expect(valueFiles).To(Equal(append(defaultValueFiles,
					"/test1.yaml",
					"/test2.yaml")))
			})
			It("Returns a default set of values and extravaluefiles with prefix", func() {
				valueFiles := newApplicationValueFiles(*pattern, "myprefix")
				Expect(valueFiles).To(Equal(append(prefixArray(defaultValueFiles, "myprefix"),
					"myprefix/test1.yaml",
					"myprefix/test2.yaml")))
			})
		})
	})
})
