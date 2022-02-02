Here be dragons

References:
- https://sdk.operatorframework.io/docs/building-operators/golang/quickstart/


Commands:
 make deploy
 oc delete pods -n patterns-operator-system --all; oc get pods -n patterns-operator-system -w 
 oc logs -n patterns-operator-system `oc get -n patterns-operator-system pods | grep -v NAME | grep Running | head -n 1 | awk '{print $1}'` -c manager -f  
