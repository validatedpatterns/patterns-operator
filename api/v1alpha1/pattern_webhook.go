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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var patternlog = logf.Log.WithName("pattern-resource")

// +kubebuilder:object:generate=false
// +k8s:deepcopy-gen=false
// +k8s:openapi-gen=false
// PatternValidator validates Pattern resources to enforce singleton semantics.
type PatternValidator struct {
	Client client.Client
}

//nolint:lll
// +kubebuilder:webhook:verbs=create,path=/validate-gitops-hybrid-cloud-patterns-io-v1alpha1-pattern,mutating=false,failurePolicy=fail,groups=gitops.hybrid-cloud-patterns.io,resources=patterns,versions=v1alpha1,name=vpattern.gitops.hybrid-cloud-patterns.io,admissionReviewVersions=v1,sideEffects=none

var _ webhook.CustomValidator = &PatternValidator{}

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *PatternValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&Pattern{}).
		WithValidator(r).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *PatternValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	p, err := convertToPattern(obj)
	if err != nil {
		return nil, err
	}
	patternlog.Info("validate create", "name", p.Name)

	var patterns PatternList
	if err = r.Client.List(ctx, &patterns); err != nil {
		return nil, fmt.Errorf("failed to list Pattern resources: %v", err)
	}
	if len(patterns.Items) > 0 {
		return nil, fmt.Errorf("only one Pattern resource is allowed")
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *PatternValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	p, err := convertToPattern(newObj)
	if err != nil {
		return nil, err
	}
	patternlog.Info("validate update", "name", p.Name)
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *PatternValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	p, err := convertToPattern(obj)
	if err != nil {
		return nil, err
	}
	patternlog.Info("validate delete", "name", p.Name)
	return nil, nil
}

func convertToPattern(obj runtime.Object) (*Pattern, error) {
	p, ok := obj.(*Pattern)
	if !ok {
		return nil, fmt.Errorf("expected a Pattern object but got %T", obj)
	}
	return p, nil
}
