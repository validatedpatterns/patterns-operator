package gitea

import (
	"context"
	"strconv"

	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/sdk/gitea"
	argoapi "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned"
	yaml "gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -source $GOFILE -package=$GOPACKAGE -destination=mock_$GOFILE

const (
	AppProjectName        = "gitea-pattern-operator"
	DefaultUser           = "hybrid-cloud-patterns"
	AdminSecretName       = "gitea-admin-credentials" // #nosec G101
	DefaultUserSecretName = "gitea-user-credentials"  // #nosec G101
	Namespace             = "gitea"
	AdminEmail            = "gitea@local.domain"
	DefaultUserEmail      = "hybrid-cloud-patterns@local.domain"
	AdminUser             = "gitea-admin"
	ReleaseName           = "gitea-pattern-operator"
	ApplicationName       = "gitea-server"
	HelmChart             = "gitea"
	HelmRepoURL           = "https://dl.gitea.io/charts/"
	RouteName             = "gitea"
	servicePort           = 3000
	chartVersion          = "6.0.5"
)

var (
	serviceName    = fmt.Sprintf("%s-http", ReleaseName)
	giteaNamespace = v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: Namespace, Labels: map[string]string{"argocd.argoproj.io/managed-by": "openshift-gitops"}}}
	route          = routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      RouteName},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: "http"}},
			TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge},
			To:  routev1.RouteTargetReference{Name: serviceName},
		}}
)

type client struct {
	cli *gitea.Client
}

func NewClient() ClientInterface {
	return &client{}
}

type ClientInterface interface {
	HasDefaultUser() (bool, error)
	CreateUser(username, password string) error
	GetClonedRepositoryURLFor(upstreamURL, revision string) (string, error)
	CloneRepository(upstreamURL, repoRef, username, password string) (string, error)
	Connect(url, username, password string) error
}

func (g *client) HasDefaultUser() (bool, error) {
	_, resp, err := g.cli.GetUserInfo(DefaultUser)
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return true, err
}

func (g *client) Connect(url, username, password string) error {
	cli, err := gitea.NewClient(url, gitea.SetBasicAuth(username, password))
	if err != nil {
		return err
	}
	g.cli = cli
	return nil
}

func (g *client) CreateUser(username, password string) error {
	_, _, err := g.cli.AdminCreateUser(gitea.CreateUserOption{LoginName: DefaultUser, Username: username, Password: password, Email: DefaultUserEmail, MustChangePassword: gitea.OptionalBool(false)})
	return err
}

func (g *client) GetClonedRepositoryURLFor(upstreamURL, reference string) (string, error) {

	upstreamRepoName, err := extractGitRepoNameFromURL(upstreamURL)
	if err != nil {
		return "", err
	}
	repo, resp, err := g.cli.GetRepo(DefaultUser, upstreamRepoName)
	if err != nil && resp.StatusCode != http.StatusNotFound {
		return "", fmt.Errorf("error while getting repository form git server:%v", err)
	}

	_, resp, err = g.cli.GetRepoBranch(DefaultUser, upstreamRepoName, reference)
	if err != nil && resp.StatusCode != http.StatusNotFound {
		return "", err
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	return repo.CloneURL, nil
}

func (g *client) CloneRepository(upstreamURL, repoRef, username, password string) (string, error) {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           upstreamURL,
		RemoteName:    "upstream",
		ReferenceName: plumbing.NewBranchReferenceName(repoRef)})
	if err != nil {
		return "", err
	}
	name, err := extractGitRepoNameFromURL(upstreamURL)
	if err != nil {
		return "", err
	}
	repo, resp, err := g.cli.GetRepo(username, name)
	if err != nil && resp.StatusCode != http.StatusNotFound {
		return "", err
	}
	if resp.StatusCode == http.StatusNotFound {
		repo, _, err = g.cli.CreateRepo(gitea.CreateRepoOption{Name: name})
		if err != nil {
			return "", err
		}
	}
	_, err = r.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{repo.CloneURL}})
	if err != nil {
		return "", err
	}
	err = r.Push(&git.PushOptions{
		RemoteName:      "origin",
		Auth:            &githttp.BasicAuth{Username: username, Password: password},
		Progress:        nil,
		Prune:           false,
		Force:           false,
		InsecureSkipTLS: false,
		CABundle:        []byte{},
	})
	if err != nil {
		return "", err
	}
	return repo.CloneURL, nil
}

func extractGitRepoNameFromURL(surl string) (string, error) {
	u, err := url.Parse(surl)
	if err != nil {
		return "", err
	}
	s := strings.Split(u.Path, "/")
	n := s[len(s)-1]
	if len(n) == 0 {
		return "", fmt.Errorf("invalid repository name for URL %s", surl)
	}
	if strings.HasSuffix(n, ".git") {
		return strings.Split(n, ".git")[0], nil
	}
	return n, nil
}

type kubeClient struct {
	cli      kclient.Client
	routeCli routev1client.Interface
}

func NewKubeClient(kubeCli kclient.Client, routeClient routev1client.Interface) *kubeClient {
	return &kubeClient{
		routeCli: routeClient,
		cli:      kubeCli,
	}
}

func (k *kubeClient) CreateNamespace() error {
	objKey := kclient.ObjectKeyFromObject(&giteaNamespace)
	ns := &v1.Namespace{}
	err := k.cli.Get(context.Background(), objKey, ns)
	if err != nil && kerrors.IsNotFound(err) {
		return k.cli.Create(context.Background(), &giteaNamespace)
	}
	return err
}

func (k *kubeClient) CreateRoute() (*routev1.Route, error) {
	ret, err := k.routeCli.RouteV1().Routes(Namespace).Create(context.Background(), &route, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (k *kubeClient) GetRoute() (*routev1.Route, error) {
	r, err := k.routeCli.RouteV1().Routes(Namespace).Get(context.Background(), RouteName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (k *kubeClient) CreateUserCredentials(username, email, secretName string) (*v1.Secret, error) {
	s, err := k.GetSecret(secretName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, err
		}
		// secret does not exist, create the admin credentials
		s, err = k.createCredentialsSecret(username, email, secretName)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (k *kubeClient) GetSecret(secretName string) (*v1.Secret, error) {
	s := v1.Secret{}
	err := k.cli.Get(context.Background(), types.NamespacedName{Name: secretName, Namespace: Namespace}, &s)

	if err != nil {
		return nil, err
	}
	return &s, nil

}
func (k *kubeClient) createCredentialsSecret(username, email, secretName string) (*v1.Secret, error) {

	s := v1.Secret{}
	err := k.cli.Get(context.Background(), types.NamespacedName{Name: secretName, Namespace: Namespace}, &s)

	if err != nil && !kerrors.IsNotFound(err) {
		return nil, err
	}
	// secret exists already, probably from a previous reconciliation loop
	if err == nil {
		return &s, nil
	}
	// secret does not exist yet, let's create one
	password, err := generatePassword(16, 4, 4, 4)
	if err != nil {
		return nil, err
	}
	s = v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: Namespace},
		Data: map[string][]byte{"username": []byte(username), "password": []byte(password), "email": []byte(email)},
	}
	err = k.cli.Create(context.Background(), &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func generateCustomChartValues(userID, groupID int, domain string) (string, error) {

	v := map[string]interface{}{
		"gitea": map[string]interface{}{
			"admin": map[string]interface{}{
				"existingSecret": AdminSecretName,
			},
			"config": map[string]interface{}{
				"server": map[string]interface{}{
					"ROOT_URL":         fmt.Sprintf("https://%s", domain),
					"START_SSH_SERVER": false,
					"DISABLE_SSH":      true,
					"HTTP_PORT":        servicePort,
				},
				"service": map[string]interface{}{
					"DISABLE_REGISTRATION": true,
				},
			},
		},
		"containerSecurityContext": map[string]interface{}{
			"allowPrivilegeEscalation": false,
			"capabilities": map[string]interface{}{
				"drop": []interface{}{
					"ALL",
				},
			},
			"runAsNonRoot": true,
			"seccompProfile": map[string]interface{}{
				"type": "RuntimeDefault",
			},
			"runAsUser": nil,
		},
		"podSecurityContext": nil,
		"image": map[string]interface{}{
			"rootless": true,
		},
		"memcached": map[string]interface{}{
			"enabled": false,
		},
		"statefulset": map[string]interface{}{
			"env": []interface{}{
				map[string]interface{}{
					"name":  "HOME",
					"value": "/data/git",
				},
			},
		},
		"persistence": map[string]interface{}{
			"size": "1Gi",
		},
		"postgresql": map[string]interface{}{
			"persistence": map[string]interface{}{
				"size": "1Gi",
			},
			"securityContext": map[string]interface{}{
				"fsGroup": groupID,
			},
			"containerSecurityContext": map[string]interface{}{
				"allowPrivilegeEscalation": false,
				"capabilities": map[string]interface{}{
					"drop": []interface{}{
						"ALL",
					},
				},
				"runAsNonRoot": true,
				"seccompProfile": map[string]interface{}{
					"type": "RuntimeDefault",
				},
				"runAsUser": userID,
			},
		},
	}

	ret, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func getIDFromAnnotation(annotations map[string]string, key string) (string, error) {
	ids, ok := annotations[key]
	if !ok {
		return "", fmt.Errorf("unable to find annotation '%s' in namespace %s", key, Namespace)
	}
	id := strings.Split(ids, "/")
	if len(id) != 2 {
		return "", fmt.Errorf("unexpected format %s", ids)
	}
	return id[0], nil
}

func (k *kubeClient) getUserGroupIDsFromNamespace() (int, int, error) {
	// due to limitations to the postgresql chart version in gitea, we can't provide a null value for  the userID and groupID in postgresql, they are defaulted to 1001 otherwise. It works for gitea though.
	// Until gitea upgrades the dependency to a newer version of postgresql that allows us to pass null for the userID and groupID, the chart has to contain valid values for these fields, which can only be retrieved from the annotation in the namespace

	ns := v1.Namespace{}
	err := k.cli.Get(context.Background(), types.NamespacedName{Name: Namespace}, &ns)
	if err != nil {
		return 0, 0, err
	}
	sgid, err := getIDFromAnnotation(ns.ObjectMeta.Annotations, "openshift.io/sa.scc.supplemental-groups")
	if err != nil {
		return 0, 0, err
	}
	suid, err := getIDFromAnnotation(ns.ObjectMeta.Annotations, "openshift.io/sa.scc.uid-range")
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(sgid)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(suid)
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil

}

func (k *kubeClient) NewApplication(domain string) (*argoapi.Application, error) {

	uid, gid, err := k.getUserGroupIDsFromNamespace()
	if err != nil {
		return nil, err
	}
	values, err := generateCustomChartValues(uid, gid, domain)
	if err != nil {
		return nil, err
	}
	return &argoapi.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: ApplicationName},
		Spec: argoapi.ApplicationSpec{
			Project: "default",
			Source: argoapi.ApplicationSource{
				RepoURL:        HelmRepoURL,
				TargetRevision: chartVersion,
				Chart:          HelmChart,
				Helm: &argoapi.ApplicationSourceHelm{
					Values:      values,
					ValueFiles:  []string{"values.yaml"},
					ReleaseName: ReleaseName,
				},
			},
			Destination: argoapi.ApplicationDestination{
				Name:      "in-cluster",
				Namespace: Namespace,
			},
			SyncPolicy: &argoapi.SyncPolicy{
				Automated: &argoapi.SyncPolicyAutomated{
					SelfHeal: true,
					Prune:    true,
				},
			},
		}}, nil

}
