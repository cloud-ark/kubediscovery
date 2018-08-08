#!/bin/bash

export GOOS=linux; go build .
cp kubediscovery ./artifacts/simple-image/kube-discovery-apiserver
docker build -t kube-discovery-apiserver:latest ./artifacts/simple-image


