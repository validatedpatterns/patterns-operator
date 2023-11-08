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
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gitopsv1alpha1 "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment
var tempLocalGitCopy, tempDir string

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

func copyFolder(srcFolder, destFolder string) error {
	srcInfo, err := os.Stat(srcFolder)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destFolder, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcFolder)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcFolder, entry.Name())
		destPath := filepath.Join(destFolder, entry.Name())

		if entry.IsDir() {
			if err := copyFolder(srcPath, destPath); err != nil {
				return err
			}
		} else {
			srcFile, err := os.Open(srcPath)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createTempDir(base string) string {
	tempDir, err := os.MkdirTemp("", base)
	Expect(err).ToNot(HaveOccurred())
	return tempDir
}

func cleanupTempDir(tempDir string) {
	err := os.RemoveAll(tempDir)
	Expect(err).ToNot(HaveOccurred())
}

func getSourceCodeFolder() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	err = gitopsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apiv1.Install(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = operatorv1.Install(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	tempDir = createTempDir("vp-test")
	tempLocalGitCopy = createTempDir("vp-checkout-test")
	cwd := getSourceCodeFolder()
	copyFolder(cwd, tempLocalGitCopy)
	err = cloneRepo(tempLocalGitCopy, tempDir, "")
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	cleanupTempDir(tempDir)
	cleanupTempDir(tempLocalGitCopy)
})
