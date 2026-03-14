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

package console

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// CatalogDeploymentName is the name of the pattern-ui-catalog Deployment
	CatalogDeploymentName = "patterns-operator-pattern-ui-catalog"
	// CatalogContainerName is the name of the container inside the catalog Deployment
	CatalogContainerName = "patterns-operator-pattern-ui-catalog"
	// CatalogConfigMapName is the name of the nginx config ConfigMap
	CatalogConfigMapName = "patterns-operator-pattern-ui-catalog"
	// CatalogServiceName is the name of the catalog Service
	CatalogServiceName = "patterns-operator-pattern-ui-catalog"
	// CatalogCertSecretName is the name of the serving cert Secret
	CatalogCertSecretName = "patterns-operator-pattern-ui-catalog-cert"
	// CatalogDefaultImage is the default catalog image
	CatalogDefaultImage = "quay.io/validatedpatterns/pattern-ui-catalog:stable-v1"
	// operatorConfigMap is the name of the operator ConfigMap (mirrors controllers.OperatorConfigMap)
	operatorConfigMap = "patterns-operator-config"
	// catalogComponentLabel is the label applied to all catalog resources
	catalogComponentLabel = "patterns-operator-pattern-ui-catalog"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch

// CreateOrUpdateCatalog creates or updates the pattern-ui-catalog ConfigMap, Service,
// and Deployment. If the operator ConfigMap contains a "catalog.image" key, that
// image is used instead of the built-in default.
func CreateOrUpdateCatalog(ctx context.Context, cl client.Client, reader client.Reader) error {
	logger := log.FromContext(ctx).WithName("catalog")
	ns := getDeploymentNamespace()

	image := getCatalogImage(ctx, reader)
	logger.Info("using catalog image", "image", image)

	if err := createOrUpdateCatalogConfigMap(ctx, cl, ns); err != nil {
		return err
	}
	if err := createOrUpdateCatalogService(ctx, cl, ns); err != nil {
		return err
	}
	if err := createOrUpdateCatalogDeployment(ctx, cl, ns, image); err != nil {
		return err
	}

	return nil
}

// getCatalogImage reads the optional "catalog.image" override from the operator
// ConfigMap. Returns the default image when the key is absent or empty.
func getCatalogImage(ctx context.Context, reader client.Reader) string {
	var cm corev1.ConfigMap
	if err := reader.Get(ctx, client.ObjectKey{
		Namespace: defaultNamespace,
		Name:      operatorConfigMap,
	}, &cm); err != nil {
		return CatalogDefaultImage
	}
	if image := cm.Data["catalog.image"]; image != "" {
		return image
	}
	return CatalogDefaultImage
}

func catalogLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/component": catalogComponentLabel,
	}
}

func createOrUpdateCatalogConfigMap(ctx context.Context, cl client.Client, namespace string) error {
	nginxConf := `error_log /dev/stdout info;
events {}
http {
  access_log         /dev/stdout;
  include            /etc/nginx/mime.types;
  default_type       application/octet-stream;
  keepalive_timeout  65;
  types {
    text/yaml         yaml yml;
  }
  server {
    listen              9444 ssl;
    listen              [::]:9444 ssl;
    ssl_certificate     /var/cert/tls.crt;
    ssl_certificate_key /var/cert/tls.key;
    root                /usr/share/nginx/html;
  }
}
`

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CatalogConfigMapName,
			Namespace: namespace,
			Labels:    catalogLabels(),
		},
		Data: map[string]string{
			"nginx.conf": nginxConf,
		},
	}

	existing := &corev1.ConfigMap{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(desired), existing); apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, desired); err != nil {
			return fmt.Errorf("could not create catalog configmap: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not check for existing catalog configmap: %w", err)
	} else {
		existing.Labels = desired.Labels
		existing.Data = desired.Data
		if err := cl.Update(ctx, existing); err != nil {
			return fmt.Errorf("could not update catalog configmap: %w", err)
		}
	}
	return nil
}

func createOrUpdateCatalogService(ctx context.Context, cl client.Client, namespace string) error {
	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CatalogServiceName,
			Namespace: namespace,
			Labels:    catalogLabels(),
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": CatalogCertSecretName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "9444-tcp",
					Port:       PatternCatalogServicePort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("https"),
				},
			},
			Selector:        catalogLabels(),
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
		},
	}

	existing := &corev1.Service{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(desired), existing); apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, desired); err != nil {
			return fmt.Errorf("could not create catalog service: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not check for existing catalog service: %w", err)
	} else {
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Selector = desired.Spec.Selector
		if err := cl.Update(ctx, existing); err != nil {
			return fmt.Errorf("could not update catalog service: %w", err)
		}
	}
	return nil
}

func createOrUpdateCatalogDeployment(ctx context.Context, cl client.Client, namespace, image string) error {
	labels := catalogLabels()
	replicas := int32(1)
	defaultMode := int32(420)
	maxUnavailable := intstr.FromString("25%")
	maxSurge := intstr.FromString("25%")

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CatalogDeploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            CatalogContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: PatternCatalogServicePort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "pattern-ui-catalog-cert",
									MountPath: "/var/cert",
									ReadOnly:  true,
								},
								{
									Name:      "nginx-conf",
									MountPath: "/etc/nginx/nginx.conf",
									SubPath:   "nginx.conf",
									ReadOnly:  true,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
					DNSPolicy:     corev1.DNSClusterFirst,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
					},
					ServiceAccountName: "patterns-operator-controller-manager",
					Volumes: []corev1.Volume{
						{
							Name: "pattern-ui-catalog-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									DefaultMode: &defaultMode,
									SecretName:  CatalogCertSecretName,
								},
							},
						},
						{
							Name: "nginx-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode: &defaultMode,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: CatalogConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	existing := &appsv1.Deployment{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(desired), existing); apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, desired); err != nil {
			return fmt.Errorf("could not create catalog deployment: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not check for existing catalog deployment: %w", err)
	} else {
		existing.Labels = desired.Labels
		existing.Spec = desired.Spec
		if err := cl.Update(ctx, existing); err != nil {
			return fmt.Errorf("could not update catalog deployment: %w", err)
		}
	}
	return nil
}

func boolPtr(b bool) *bool {
	return &b
}
