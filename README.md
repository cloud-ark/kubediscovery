# kubediscovery

A Kubernetes Aggregated API Server that helps with discovery of dynamic information about your cluster.


## What is it?

kubediscovery is a tool that helps you find dynamic composition tree of Kubernetes Objects. 
In Kubernetes there are top-level resources which are composed of other resources. 
For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods. 
kubediscovery is a Kubernetes Aggregated API Server that helps you find composition trees of Kubernetes objects.


## How does it work?

You provide it a YAML file that defines static composition relationship between different Resources/Kinds.
Using this information kubediscovery API Server builds the dynamic composition information by 
continuously querying the Kubernetes API for various Objects of different Kinds that are created in your cluster.

The YAML file can contain both in-built Kinds (such as Deployment, Pod, Service), and
Custom Resource Kinds (such as EtcdCluster).
kubedsicovery API server registers REST endpoints for all the kinds that are defined in the YAML file.
These endpoints will be accessed by `kubectl get` command when you want to retrieve the dynamic
composition information (see examples below). An example YAML file is provided (kind_compositions.yaml).
There is also kind_compositions.yaml.with-etcd which shows definition for the EtcdCluster custom resource.
Use this YAML only after you deploy the [Etcd Operator](https://github.com/coreos/etcd-operator)
(Rename this file to kind_compositions.yaml before deploying the API server).

The dynamic composition information is currently collected for the "default" namespace.
It is stored in memory. In the future we will store it in the Etcd instance that we run along with
the API server. We use OwnerReferences to build the dynamic composition tree for Objects.
For querying the main API server, we use direct REST calls instead of typed clients. 
This is done because we want to be able to query for Objects 
based on what is defined in kind_compositions.yaml, which we won't know in advance.
So we cannot use typed clients inside kubediscovery to query the main API server to build the dynamic composition tree.

In building this API server we tried several approaches. You can read about our experience  
[here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).


## How is it different than..

```
kubectl get all
```

1) Using kubediscovery you can find out composition tree of specific Kinds.

2) You can find composition trees of all objects or a specific Object of a particular Kind.

3) Works with Custom Resources.


## Try it on Minikube


Scripts are provided to help with building the API server container image and deployment/cleanup.

0) Allow Minikube to use local Docker images: 

   `$ eval $(minikube docker-env)`

1) Install/Vendor in dependencies:

   `$ dep ensure`

2) Build the API Server container image:

   `$ ./build-discovery-artifacts.sh`

3) Deploy the API Server in your cluster:

   `$ ./deploy-discovery-artifacts.sh`

4) Clean-up:

    `$ ./delete-discovery-artifacts.sh`


Once the kubediscovery API server is running, you can find the dynamic composition information by using following type of commands:


1) Get dynamic composition for all deployments

```
kubectl get --raw /apis/kubediscovery.cloudark.io/v1/namespaces/default/deployments/*/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/all-deployments.png)


2) Get dynamic composition for a particular deployment

```
kubectl get --raw /apis/kubediscovery.cloudark.io/v1/namespaces/default/deployments/<dep-name>/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/single-deployment.png)


3) Get dynamic composition of all etcdclusters custom resource (if etcdclusters custom resource is registered in the cluster)

```
kubectl get --raw /apis/kubediscovery.cloudark.io/v1/namespaces/default/etcdclusters/*/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/etcd-clusters.png)

You can use above style of commands with all the Kinds that you have defined in kind_compositions.yaml


## Troubleshooting tips:

1) Check that the API server Pod is running: 

   `$ kubectl get pods -n discovery`

2) Get the Pod name from output of above command and then check logs of the container.
   For example:

   `$ kubectl logs -n discovery kube-discovery-apiserver-kjz7p  -c kube-discovery-apiserver`


### References:

The Aggregated API Server has been developed by refering to [sample-apiserver](https://github.com/kubernetes/sample-apiserver)
and [custom-metrics-apiserver](https://github.com/kubernetes-incubator/custom-metrics-apiserver).
