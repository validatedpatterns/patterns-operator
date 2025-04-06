// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"

	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	scheme "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// AppProjectsGetter has a method to return a AppProjectInterface.
// A group's client should implement this interface.
type AppProjectsGetter interface {
	AppProjects(namespace string) AppProjectInterface
}

// AppProjectInterface has methods to work with AppProject resources.
type AppProjectInterface interface {
	Create(ctx context.Context, appProject *v1alpha1.AppProject, opts v1.CreateOptions) (*v1alpha1.AppProject, error)
	Update(ctx context.Context, appProject *v1alpha1.AppProject, opts v1.UpdateOptions) (*v1alpha1.AppProject, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.AppProject, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.AppProjectList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AppProject, err error)
	AppProjectExpansion
}

// appProjects implements AppProjectInterface
type appProjects struct {
	*gentype.ClientWithList[*v1alpha1.AppProject, *v1alpha1.AppProjectList]
}

// newAppProjects returns a AppProjects
func newAppProjects(c *ArgoprojV1alpha1Client, namespace string) *appProjects {
	return &appProjects{
		gentype.NewClientWithList[*v1alpha1.AppProject, *v1alpha1.AppProjectList](
			"appprojects",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1alpha1.AppProject { return &v1alpha1.AppProject{} },
			func() *v1alpha1.AppProjectList { return &v1alpha1.AppProjectList{} }),
	}
}
