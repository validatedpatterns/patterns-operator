Here be dragons

References:
- https://sdk.operatorframework.io/docs/building-operators/golang/quickstart/


## Deploy the operator
```
 make deploy
```
This creates a new Deployment that points to the pre-built image on quay

## Create the Multi-Cloud GitOps pattern

```
kubectl create -f config/samples/gitops_v1alpha1_pattern.yaml
```

### Check the status section
```
kubectl get -f config/samples/gitops_v1alpha1_pattern.yaml -o yaml
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
 oc logs -n patterns-operator-system `oc get -n patterns-operator-system pods | grep -v NAME | grep Running | head -n 1 | awk '{print $1}'` -c manager -f  
```

## Development
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
make docker-build docker-push
```

Next, create the OperatorHub release:

```
CHANNELS=fast make bundle

git clone git@github.com:$USER/community-operators-prod.git
rsync -a bundle/ community-operators-prod/operators/patterns-operator/$VERSION/
git add community-operators-prod/operators/patterns-operator/$VERSION/
git commit -s -m "New v$VERSION validated patterns operator release"
git push

echo "Now create a PR against https://github.com/redhat-openshift-ecosystem/community-operators-prod"
```
