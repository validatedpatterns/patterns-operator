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
	"bytes"
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/ghodss/yaml"
)

func haveNamespace(client kubernetes.Interface, name string) bool {
	if _, err := client.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{}); err == nil {
		return true
	}
	return false
}

func havePod(client kubernetes.Interface, namespace, pod string) bool {
	if _, err := client.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{}); err == nil {
		return true
	}
	return false
}

func haveContainer(client kubernetes.Interface, namespace, pod, container string) bool {
	pods, err := client.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
	if err != nil {
		return false
	}
	for i := range pods.Spec.Containers {
		if pods.Spec.Containers[i].Name == container {
			return true
		}
	}
	return false
}

func execInPod(config *rest.Config, client kubernetes.Interface, namespace, pod, container string, cmd []string) (*bytes.Buffer, *bytes.Buffer, error) {
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod).Namespace(namespace).SubResource("exec").Param("container", container)
	req.VersionedParams(
		&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		},
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	return &stdout, &stderr, err
}

func ownedBySame(expected, object metav1.Object) bool {
	ownerReferences := expected.GetOwnerReferences()

	for _, r := range ownerReferences {
		if !ownedBy(object, r) {
			return false
		}
	}
	return true
}

func ownedBy(object metav1.Object, ref metav1.OwnerReference) bool {

	ownerReferences := object.GetOwnerReferences()

	for _, r := range ownerReferences {
		if referSameObject(r, ref) {
			return true
		}
	}

	return false
}

func objectYaml(object metav1.Object) string {

	if yamlString, err := yaml.Marshal(object); err != nil {
		return fmt.Sprintf("Error marshalling object: %s\n", err.Error())
	} else {
		return string(yamlString)
	}
}

// Returns true if a and b point to the same object.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}
