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