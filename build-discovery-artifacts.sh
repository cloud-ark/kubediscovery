#!/bin/bash

export GOOS=linux; go build .
cp kubediscovery ./artifacts/simple-image/kube-discovery-apiserver
docker build -t lmecld/kube-discovery-apiserver:0.4.1 ./artifacts/simple-image


