# kubediscovery

Kubediscovery enables discovering dynamic information about Kubernetes resources in your Kubernetes cluster. It supports both built-in resources and Custom resources. 

<!-- ![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/kubediscovery.jpg =50x50) -->

Kubediscovery's initial implementation was as a Kubernetes Aggregated API Server. 
You can read about that approach in [this blog post](https://medium.com/@cloudark/kubediscovery-aggregated-api-server-to-learn-more-about-kubernetes-custom-resources-18202a1c4aef). When building this Aggregated API server based implementation we tried several approaches. You can read about that experience [here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).

## Profiling

- Modify main.go to adjust to profiling flag input
- go build .
- ./kubediscovery -cpuprofile=abc.prof connections Pod etcd-minikube kube-system
- go tool pprof kubediscovery abc.prof
- top5 -cum
- https://blog.golang.org/pprof

## How it works?

### Composition

The ‘composition’ function of Kubediscovery provides a way to obtain dynamic composition tree of a Kubernetes resource instance in terms of its underlying resource instances.

In Kubernetes, certain resources are organized in a hierarchy — a parent resource is composed using its underlying resource/s. For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods. Similarly, a Custom Resource such as Postgres can be composed of a Deployment and a Service resource. To build the dynamic composition tree, Kubediscovery needs static information about resource hierarchy of a Custom Resource. CRD/Operator developers needs to follow certain guidelines during development that will help with providing this information.
We have detailed these guidelines [here](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md). The two key guidelines in the context of 'composition' functionality are: 
a) Define underlying resources that are part of a Custom Resource’s resource hierarchy with an Annotation on CRD registration YAML ([guideline #23](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#expose-custom-resource-composition-information)),
and b) Set OwnerReferences for underlying resources owned by your 
Custom Resource ([guideline #15](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#set-ownerreferences-for-underlying-resources-owned-by-your-custom-resource)).

Using the static hierarchy information kubediscovery builds the dynamic composition trees by following OwnerReferences of individual resource instances.

### Connections

The 'connections' function of Kubediscovery provides a way to obtain dynamic resource relationships between Kubernetes resources that are based on labels, annotations, spec properties and environment variables. CRD/Operator developer need to define these relationships on the CRDs. See [this guideline](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#document-labels-annotations-or-spec-property-based-dependencies-for-your-custom-resources)

### Man

The 'man page' functionality of Kubediscovery provides a way to obtain 'man page' like information about a Kubernetes resource. CRD/Operator developer needs to package this information as a ConfigMap and include it in their Operator's Helm chart. See [this guideline](https://github.com/cloud-ark/kubeplus/blob/master/Guidelines.md#define-man-page-for-your-custom-resources)


## Try it

Download Minikube
- You will need VirtualBox installed
- Download appropriate version of Minikube for your platform

1) Deploy Nginx Pod:

   `$ kubectl apply -f nginx-deployment.yaml`

2) Get composition trees for a Deployment

```
./kubediscovery composition Deployment <name> default
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/nginx-deployment-composition.png)


3) Get composition trees for a ReplicaSet

```
./kubediscovery composition ReplicaSet <name> default
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/replicaset-composition.png)


4) Get composition trees for a Pod

```
./kubediscovery composition Pod <pod-name> default
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/all-pod-composition.png)


5) Delete nginx deployment

```
kubectl delete -f nginx-deployment.yaml
```


6) Get Postgres composition tree

This will work only after you have deployed Postgres Operator. 
Follow [these steps](https://github.com/cloud-ark/kubeplus/blob/master/kubeplus-steps.txt) to deploy Postgres Operator.

```
./kubediscovery Postgres postgres1 default
```

![alt text](https://github.com/cloud-ark/kubediscovery/raw/master/docs/postgres-composition.png)



## Development

1) Start Minikube 

2) Allow Minikube to use local Docker images: 

   `$ eval $(minikube docker-env)`

3) Use Go version 1.10.2 or above

4) Install/Vendor in dependencies:

   `$ dep ensure`

5) Build Kubediscovery executable:

   `$ go build .`

6) Try:
   
   Follow the steps listed under 'Try it' section above.


### Issues/Suggestions

Issues and suggestions for improvement are welcome. 
Please file them [here](https://github.com/cloud-ark/kubediscovery/issues)



