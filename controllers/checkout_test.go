package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Git Functions", func() {

	Context("checkoutRevision", func() {
		It("should checkout a specific commit", func() {
			err := checkoutRevision(tempDir, "", "3086ab9e72e9f9ea369813c76f35772a3c8ea2a4") //some older existing commit hash
			Expect(err).To(BeNil())
		})
	})

	Context("cloneRepo", func() {
		It("should clone a repository and get the HEAD", func() {
			refHash, err := repoHash(tempDir)
			Expect(err).To(BeNil())
			Expect(refHash).ToNot(BeNil())
		})
	})

	Context("repoHash", func() {
		It("should get the repository hash", func() {
			refHash, err := repoHash(tempDir)
			Expect(err).To(BeNil())
			Expect(refHash).ToNot(BeNil())
		})
	})
})
