/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	//	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

func TestGit(t *testing.T) {
	RegisterFailHandler(Fail)
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var err error
	branch := "main"
	directory := "/tmp/git-test"
	if _, err = os.Stat(fmt.Sprintf("%s/.git", directory)); os.IsNotExist(err) {
		err = cloneRepo("https://github.com/hybrid-cloud-patterns/multicloud-gitops", directory, "")
		Expect(err).NotTo(HaveOccurred())
	}

	logf.Log.Info("Accessing", "directory", directory)
	repo, err := git.PlainOpen(directory)
	Expect(err).NotTo(HaveOccurred())

	input := plumbing.NewBranchReferenceName(branch)
	b, err := repo.Storer.Reference(input)
	logf.Log.Info("created reference", "branch", b, "input", input)
	Expect(err).NotTo(HaveOccurred())

	err = checkoutRevision(directory, "", branch)
	Expect(err).NotTo(HaveOccurred())

	err = checkoutRevision(directory, "", "d1578208f326431805d56a96491f3312f1fa4658")
	Expect(err).NotTo(HaveOccurred())

	err = checkoutRevision(directory, "", "d157820")
	Expect(err).To(HaveOccurred())

}
