Here be dragons

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

### Initialize the secrets vault

```
wget https://raw.githubusercontent.com/hybrid-cloud-patterns/common/main/scripts/vault-utils.sh
bash ./vault-utils.sh vault_init ./pattern-vault.init
```

### Delete the pattern

```
kubectl delete -f config/samples/gitops_v1alpha1_pattern.yaml
```

This will only remove the top-level application.
The subscription and anything created by Argo will not be removed and canmust be removed manually.
Removing the top-level application ensures that Argo won't try to put back anything you delete.

## Watch the logs
```
oc logs -npatterns-operator-system `oc get -npatterns-operator-system pods -o name --field-selector status.phase=Running | grep patterns` -c manager -f
```

## Development

### Test your changes
```
BRANCH=`whoami`
git co -b $BRANCH
vi somefile.go
git commit
git push --set-upstream origin $BRANCH
# Wait for quay to build
VERSION=$BRANCH make deploy
```

### Test your changes (alt)

Replace $USER and the version of the operator:
```
vi somefile.go
export IMAGE_TAG_BASE=quay.io/$USER/patterns-operator
export IMG=quay.io/$USER/patterns-operator:0.0.2
make docker-build docker-push bundle
make deploy
```


Restart the container to pick up the latest image from quay
```
 oc delete pods -n patterns-operator-system --all; oc get pods -n patterns-operator-system -w
```

### Upgrade testing with OLM

Assuming the previous version was `0.0.1`, and we're not deploying to the official Quay repository, start by defining the version, creating the 3 images, and pushing them to quay:

```
export VERSION=0.0.2
IMAGE_TAG_BASE=quay.io/$USER/patterns-operator CHANNELS=fast make docker-build docker-push bundle bundle-build bundle-push catalog-build catalog-push
```

Now create the CatalogSource so the cluster can see the new version

```
make catalog-install
```


### Releases

First define the version and create the operator image:

```
export VERSION=0.0.3
git tag $VERSION
git push $VERSION
```

Next, create the OperatorHub release:

```
CHANNELS=fast make bundle

git clone git@github.com:$USER/community-operators-prod.git
git checkout -b "patterns-operator-v$VERSION"
rsync -a bundle/ community-operators-prod/operators/patterns-operator/$VERSION/
cd community-operators-prod
git add operators/patterns-operator/$VERSION/
git commit -s -m "New v$VERSION validated patterns operator release"
git push <fork-remote> "patterns-operator-v$VERSION"
cd operators/patterns-operator
# Inspect the diff from the previously released version
diff -urN $(ls -1tr | grep -v ci.yaml | head -n2 | sort)

echo "Now create a PR against https://github.com/redhat-openshift-ecosystem/community-operators-prod"
```
