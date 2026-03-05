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

package v1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateCreate_AllowsFirstPattern(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := &PatternValidator{Client: fakeClient}

	p := &Pattern{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pattern",
			Namespace: "default",
		},
		Spec: PatternSpec{
			ClusterGroupName: "hub",
			GitConfig: GitConfig{
				TargetRepo:     "https://github.com/example/repo",
				TargetRevision: "main",
			},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), p)
	if err != nil {
		t.Errorf("expected no error for first pattern, got: %v", err)
	}
	if warnings != nil {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestValidateCreate_DeniesSecondPattern(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	existing := &Pattern{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pattern",
			Namespace: "default",
		},
		Spec: PatternSpec{
			ClusterGroupName: "hub",
			GitConfig: GitConfig{
				TargetRepo:     "https://github.com/example/repo",
				TargetRevision: "main",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	validator := &PatternValidator{Client: fakeClient}

	p := &Pattern{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "second-pattern",
			Namespace: "default",
		},
		Spec: PatternSpec{
			ClusterGroupName: "hub",
			GitConfig: GitConfig{
				TargetRepo:     "https://github.com/example/repo2",
				TargetRevision: "main",
			},
		},
	}

	_, err := validator.ValidateCreate(context.Background(), p)
	if err == nil {
		t.Error("expected error when creating second pattern, got nil")
	}
}

func TestValidateCreate_RejectsNonPatternObject(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := &PatternValidator{Client: fakeClient}

	notAPattern := &PatternList{}

	_, err := validator.ValidateCreate(context.Background(), notAPattern)
	if err == nil {
		t.Error("expected error for non-Pattern object, got nil")
	}
}

func TestValidateUpdate_Allows(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := &PatternValidator{Client: fakeClient}

	p := &Pattern{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pattern",
			Namespace: "default",
		},
	}

	warnings, err := validator.ValidateUpdate(context.Background(), p, p)
	if err != nil {
		t.Errorf("expected no error on update, got: %v", err)
	}
	if warnings != nil {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestValidateDelete_Allows(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := &PatternValidator{Client: fakeClient}

	p := &Pattern{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pattern",
			Namespace: "default",
		},
	}

	warnings, err := validator.ValidateDelete(context.Background(), p)
	if err != nil {
		t.Errorf("expected no error on delete, got: %v", err)
	}
	if warnings != nil {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}
