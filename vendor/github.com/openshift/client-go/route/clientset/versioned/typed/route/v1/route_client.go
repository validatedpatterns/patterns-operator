// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	http "net/http"

	routev1 "github.com/openshift/api/route/v1"
	scheme "github.com/openshift/client-go/route/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type RouteV1Interface interface {
	RESTClient() rest.Interface
	RoutesGetter
}

// RouteV1Client is used to interact with features provided by the route.openshift.io group.
type RouteV1Client struct {
	restClient rest.Interface
}

func (c *RouteV1Client) Routes(namespace string) RouteInterface {
	return newRoutes(c, namespace)
}

// NewForConfig creates a new RouteV1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*RouteV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new RouteV1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*RouteV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &RouteV1Client{client}, nil
}

// NewForConfigOrDie creates a new RouteV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *RouteV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new RouteV1Client for the given RESTClient.
func New(c rest.Interface) *RouteV1Client {
	return &RouteV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := routev1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = rest.CodecFactoryForGeneratedClient(scheme.Scheme, scheme.Codecs).WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *RouteV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
