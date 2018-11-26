# kubediscovery

Kubediscovery is a Kubernetes Aggregated API server that offers new API endpoints for delivering additional information about Custom Resources in your Kubernetes Cluster. Custom Resources are typically registered in a cluster as part of
installing a Custom Resource Definition (CRD) or Kubernetes Operator in the cluster.

Current implementation supports following endpoints.

* composition - Retrieve dynamic composition tree of Custom Resource instance in terms of the underlying resource instances (e.g. which underlying resource instances are part of the composition tree of a Postgres Custom Resource instance.) 
[/apis/kubeplus.cloudark.io/v1/composition](https://github.com/cloud-ark/kubeplus/blob/master/examples/moodle/steps.txt#L71)

* explain - Retrieve OpenAPI Spec for registered Custom Resources. 
[/apis/kubeplus.cloudark.io/v1/explain](https://github.com/cloud-ark/kubeplus/blob/master/examples/mysql/steps.txt#L53)


![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/kubediscovery.png)


Kubediscovery can be used in two ways - standalone or as part of 
[KubePlus Platform Toolkit](https://github.com/cloud-ark/kubeplus). In standalone mode only the 'composition' endpoint is available whereas when using with KubePlus both 'composition' and 'explain' endpoints are available.

You can read more about our goals with Kubediscovery in 
[this blog post](https://medium.com/@cloudark/kubediscovery-aggregated-api-server-to-learn-more-about-kubernetes-custom-resources-18202a1c4aef).



## How it works?

The ‘composition’ endpoint is used for obtaining dynamic composition tree of a Custom Resource instance in terms of its underlying resource instances.

In Kubernetes, certain resources are organized in a hierarchy — a parent resource is composed using its underlying resource/s. For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods. Similarly, a Custom Resource such as Postgres can be composed of a Deployment and a Service resource.

To build the dynamic composition tree, Kubediscovery needs static information about resource hierarchy of a Custom Resource. 
In standalone mode this information is provided through a YAML file that defines the hierarchical relationship between different Resources/Kinds. The YAML file can contain both in-built Kinds (such as Deployment, Pod, Service), 
and Custom Resource Kinds (such as Postgres or EtcdCluster). An example YAML file is provided (kind_compositions.yaml). There is also kind_compositions.yaml.with-etcd which shows definition for the EtcdCluster custom resource. Use this YAML only after you deploy the [Etcd Operator](https://github.com/coreos/etcd-operator) (Rename this file to kind_compositions.yaml before deploying the API server).

When using with KubePlus, CRD/Operator developers needs to follow certain guidelines during development that
will help with providing this information.
We have detailed these guidelines [here](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md). 
The two key guidelines in the context of Kubediscovery are: 
a) Define underlying resources that are part of a Custom Resource’s resource hierarchy with an Annotation on CRD registration YAML ([guideline #9](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#9-define-underlying-resources-created-by-custom-resource-as-annotation-on-crd-registration-yaml)),
and b) Set OwnerReferences for underlying resources owned by your 
Custom Resource ([guideline #5](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#5-set-ownerreferences-for-underlying-resources-owned-by-your-custom-resource)).

Using the static hierarchy information kubediscovery builds the dynamic composition trees by 
following OwnerReferences of individual resource instances and builds the dynamic composition tree.

kubediscovery supports two query parameters: `kind` and `instance` for 'composition' endpoint.

To retrieve dynamic composition tree for a particular Kind you would use following call:

```kubectl get --raw "/apis/kubeplus.cloudark.io/v1/composition?kind=Deployment&instance=nginx-deployment```

The value for `kind` query parameter should be the exact name of a Kind such as 'Deployment' and not 'deployment' or 'deployments'.

The value for `instance` query parameter should be the name of the instance. 
A special value of `*` is supported for the `instance` query parameter to retrieve 
composition trees for all instances of a particular Kind.

The dynamic composition information is currently collected for the "default" namespace only.
The work to support all namespaces is being tracked [here](https://github.com/cloud-ark/kubediscovery/issues/16).

Constructed dynamic composition trees are currently stored in memory.
If the kubediscovery pod is deleted this information will be lost.
But it will be recreated once you redeploy kubediscovery API Server.


In building this API server we tried several approaches. You can read about our experience  
[here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).


## How is it different than..

```
kubectl get all
```

1) Using kubediscovery you can find dynamic composition trees for native Kinds and Custom Resources alike.

2) You can find dynamic composition trees for a specific object or all objects of a particular Kind.


## Try it:

Download Minikube
- You will need VirtualBox installed
- Download appropriate version of Minikube for your platform
- Kubediscovery has been tested with Minikube-0.25 and Minikube-0.28.
  It should work with other versions as well. Please file an Issue if it does not.

1) Start Minikube

2) Deploy the API Server in your cluster:

   `$ ./deploy-discovery-artifacts.sh`

3) Check that API Server is running:

   `$ kubectl get pods -n discovery`    

4) Deploy Nginx Pod:

   `$ kubectl apply -f nginx-deployment.yaml`


5) Get composition trees for all deployments

```
kubectl get --raw "/apis/kubeplus.cloudark.io/v1/composition?kind=Deployment&instance=*" | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/nginx-deployment-composition.png)



6) Get composition trees for all replicasets

```
kubectl get --raw "/apis/kubeplus.cloudark.io/v1/composition?kind=ReplicaSet&instance=*" | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/replicaset-composition.png)


7) Get composition trees for all pods

```
kubectl get --raw "/apis/kubeplus.cloudark.io/v1/composition?kind=Pod&instance=*" | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/all-pod-composition.png)


8) Delete nginx deployment

```
kubectl delete -f nginx-deployment.yaml
```

9) Try getting dynamic compositions for various kinds again (repeat steps 5-7)

You can use above style of commands with all the Kinds that you have defined in kind_compositions.yaml



10) Get Postgres composition tree

This will work only after you have deployed Postgres Operator. 
Follow [these steps](https://github.com/cloud-ark/kubeplus/blob/master/kubeplus-steps.txt) to deploy Postgres Operator.

```
kubectl get --raw "/apis/kubeplus.cloudark.io/v1/composition?kind=Postgres&instance=postgres1" | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/postgres-composition.png)



## Development

1) Start Minikube 

2) Allow Minikube to use local Docker images: 

   `$ eval $(minikube docker-env)`

3) Install/Vendor in dependencies:

   `$ dep ensure`

4) Build the API Server container image:

   `$ ./build-local-discovery-artifacts.sh`

5) Deploy the API Server in your cluster:

   `$ ./deploy-local-discovery-artifacts.sh`

6) Follow steps 3-10 listed under Try it section above.




## Troubleshooting tips:

1) Check that the API server Pod is running: 

   `$ kubectl get pods -n discovery`

2) Get the Pod name from output of above command and then check logs of the container.
   For example:

   `$ kubectl logs -n discovery kube-discovery-apiserver-kjz7p  -c kube-discovery-apiserver`


### Clean-up:

  `$ ./delete-discovery-artifacts.sh`



### Issues/Suggestions:

Issues and suggestions for improvement are welcome. 
Please file them [here](https://github.com/cloud-ark/kubediscovery/issues)



