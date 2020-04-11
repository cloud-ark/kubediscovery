#!/bin/bash

if (( $# < 1 )); then
    echo "./build-artifact.sh <latest | versioned>"
fi

artifacttype=$1

if [ "$artifacttype" = "latest" ]; then
    export GOOS=linux; go build .
    cp kubediscovery ./artifacts/simple-image/kube-discovery-apiserver
    docker build -t lmecld/kube-discovery-apiserver:latest ./artifacts/simple-image
    docker push lmecld/kube-discovery-apiserver:latest
fi

if [ "$artifacttype" = "versioned" ]; then
    version=`tail -1 versions.txt`
    echo "Building version $version"
    export GOOS=linux; go build .
    cp kubediscovery ./artifacts/simple-image/kube-discovery-apiserver
    docker build -t lmecld/kube-discovery-apiserver:$version ./artifacts/simple-image
    docker push lmecld/kube-discovery-apiserver:$version
fi



