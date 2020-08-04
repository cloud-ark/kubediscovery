#!/bin/bash

#kubectl delete ns wardle
#kubectl delete -f artifacts/example/auth-delegator.yaml -n kube-system
#kubectl delete -f artifacts/example/auth-reader.yaml -n kube-system
#kubectl delete -f artifacts/example/apiservice.yaml

kubectl create -f artifacts/example/ns.yaml
kubectl create configmap -n discovery kind-compositions-config-map --from-file=kind_compositions.yaml


kubectl create -f artifacts/example/sa.yaml -n discovery
kubectl create -f artifacts/example/auth-delegator.yaml -n kube-system
kubectl create -f artifacts/example/auth-reader.yaml -n kube-system
kubectl create -f artifacts/example/grant-cluster-admin.yaml
kubectl create -f artifacts/example/rc.yaml -n discovery
kubectl create -f artifacts/example/service.yaml -n discovery
kubectl create -f artifacts/example/apiservice.yaml