Relationship Modeling
----------------------

Q. Where to define relationships?
A. On the Resources that need to react to certain annotations or labels being
added to other resources. This enables finding out this information easily.
For example, adding the relationship information on Multus provides easy way
to discover this information through 'kubectl man Multus'.

Q. How to query relationship?
A. Relationship information can be queried from both sides. 
kubectl relationship Pod <pod-name> <namespace>
kubectl relationship Service <service-name> <namespace>

kubectl relationship Multus <multus-name> <namespace>
kubectl relationship Pod <pod-name> <namespace>

The relationship between Service and Pod will be defined on Service side. 
The relationship between Multus and Pod will be defined on Multus side.

When the relationship query is done with the target resource as input (e.g.: Pod)
we need to search for all the instances of resource types that have Pod as the target and
check if the relationship information holds. This search should be limited to namespace.

