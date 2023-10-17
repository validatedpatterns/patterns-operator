[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Here be dragons!

References:
- https://sdk.operatorframework.io/docs/building-operators/golang/quickstart/

Follow https://sdk.operatorframework.io/docs/installation/ to install the operator-sdk


## Deploy the operator

Search OperatorHub for "pattern" and accept all the defaults

## Create the Multi-Cloud GitOps pattern

```
kubectl create -f config/samples/gitops_v1alpha1_pattern.yaml
```

### Check the status
```
kubectl get -f config/samples/gitops_v1alpha1_pattern.yaml -o yaml
oc get applications -A -w
```

### Load secrets into the vault

In order to load the secrets out of band into the vault you can copy the
`values-secret.yaml.template` inside the pattern's git repo to
`~/values-secret.yaml`, edit the secrets at your discretion and then run `make
load-secrets`. Otherwise you can access the vault via its network route, login
via the root token (contained in the `imperative` namespace in the `vaultkeys`
secret and then add the secrets via the UI (this approach is a bit more work)

### Delete the pattern

```
kubectl delete -f config/samples/gitops_v1alpha1_pattern.yaml
```

This will only remove the top-level application.
The subscription and anything created by Argo will not be removed and canmust be removed manually.
Removing the top-level application ensures that Argo won't try to put back anything you delete.

## Watch the logs

Note that when installing via UI the namespace will be `openshift-operators` and not `patterns-operator-system`
```
oc logs -npatterns-operator-system `oc get -npatterns-operator-system pods -o name --field-selector status.phase=Running | grep patterns` -c manager -f
```

## Development

### Test your changes locally against a remote cluster

Run the operator on your machine from your local directory against a cluster's
API.

```bash
oc login
oc apply -f ./config/crd/bases
# For Linux amd64
make run
# For MacOS arm64 (M series)
GOOS=darwin GOARCH=arm64 make run
```

#### Test your changes on a cluster (Maintainers only)

Run the operator in a Pod on an OpenShift cluster. This will create a container
image under the *hybridcloudpatterns* organization in Quay.

**NOTE:** This method only works for maintainers who have push access to the
GitHub repository. To test changes in a forked repo, see *Test your changes on
a cluster (Anyone)* below.

```bash
BRANCH=`whoami`
git co -b $BRANCH
vi somefile.go
git commit
git push --set-upstream origin $BRANCH
# Wait for quay to build
VERSION=$BRANCH make deploy
```

#### Test your changes on a cluster (Anyone)

Run the operator in a Pod on an OpenShift cluster. This will create a container
image under your user account in Quay.

**NOTE:** If you're doing this for the first time, the repo in Quay will be set
to private. You will need to change the permission on the repo to public before
running `make deploy`.

Replace $USER and the version of the operator.

```bash
vi somefile.go
export IMAGE_TAG_BASE=quay.io/$USER/patterns-operator
export IMG=quay.io/$USER/patterns-operator:0.0.2
make docker-build docker-push bundle
make deploy
```

Restart the container to pick up the latest image from quay

```bash
oc delete pods -n patterns-operator-system --all; oc get pods -n patterns-operator-system -w
```

### Validating end-to-end installation and upgrade path with OLM

The patterns operator is distributed through OLM from the official
[Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod)
catalog. It is a good idea to end-to-end validate OLM bundle changes before
releasing a new version.

The commands below will generate an operator controller image, OLM bundle
image, and OLM catalog image under your user account in Quay. The generated
catalog can then be installed on an OpenShift cluster which will provide an
option to install the candidate operator version from OperatorHub.

**NOTE:** If you're doing this for the first time, the repos in Quay will be
set to private. You will need to change the permission on the repos to public
before running `make catalog-install`.

Assuming the previous version was `0.0.1`, start by defining the version,
creating the 3 images, and pushing them to quay:

```
export USER=replace-me  # Replace user
export VERSION=0.0.2    # Replace version
IMAGE_TAG_BASE=quay.io/$USER/patterns-operator CHANNELS=fast make docker-build docker-push bundle bundle-build bundle-push catalog-build catalog-push
```

**NOTE:** If you run into errors with the `opm` command, upgrade your installed
opm version. opm release [1.26.5](https://github.com/operator-framework/operator-registry/releases/tag/v1.26.5)
or newer should be good.

Now create the CatalogSource on your cluster:

```
make catalog-install
```

After ~60 seconds, the CatalogSource object should have the status *READY*. (It
may briefly show the status *TRANSIENT_FAILURE* before showing *READY*.)

In the OpenShift Console, navigate to OperarorHub. Search for *Patterns
Operator*. Install the operator from the source *Test Patterns Operator*.

### Releases

As a first step, make sure you have already cloned the community-operators-prod via `git clone git@github.com:$USER/community-operators-prod.git`
and that it is up-to-date:
```
# First make sure community-operators-prod is uptodate
cd community-operators-prod
git fetch --all; git checkout main; git pull
```

Then switch to the `patterns-operator` git folder, define the version and create the operator image:

```
cd ../patterns-operator
export VERSION=0.0.5
git checkout -b "patterns-operator-v$VERSION"
CHANNELS=fast make bundle
git commit -a -m "Upgrade version to ${VERSION}"
gh pr create
# Merge the PR
git checkout main
git pull
git tag $VERSION
git push <upstream-remote> $VERSION

# Starting 2023-09-01 we do not let quay rebuilt the container. It is being built by a workflow
# triggered when pushing a numeric tag (x.y.z). So check the result of the 'vp-patterns/update-quay-image'
# GH action.

# Sync the bundle/ folder to the community-operators-prod git repo
rsync -va bundle/ ../community-operators-prod/operators/patterns-operator/$VERSION
```

Next, create the OperatorHub release, by creating the community operator PR:

```
cd ../community-operators-prod
git checkout -b "patterns-operator-v$VERSION"
git add operators/patterns-operator/$VERSION/
git commit -s -m "operator patterns-operator ($VERSION)"
git push <fork-remote> "patterns-operator-v$VERSION"

# Inspect the diff from the previously released version
cd operators/patterns-operator
diff -urN $(ls -1rv | grep -v ci.yaml | head -n2 | sort)

# Now create a PR against https://github.com/redhat-openshift-ecosystem/community-operators-prod
# Use the web interface so you can fill in the web template
# Create the PR and make sure you flag the questions under `Updated to existing Operators`
# and section `Your submission should not`
# Example PR https://github.com/redhat-openshift-ecosystem/community-operators-prod/pull/1569
# The PR will get automatically merged once CI passes and the PR is pushed by one of the OWNERS of the patterns-operator
# subfolder inside community-operators-prod
```
