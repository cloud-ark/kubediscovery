package discovery

import (
	"sync"
	"strings"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Used for unmarshalling JSON output from the main API server
type composition struct {
	Kind        string   `yaml:"kind"`
	Plural      string   `yaml:"plural"`
	Endpoint    string   `yaml:"endpoint"`
	Composition []string `yaml:"composition"`
}

// Used for Final output
type Composition struct {
	Level     int
	Kind      string
	Name      string
	Namespace string
	Status    string
	Children  []Composition
}

// Used to store information queried from the main API server
type MetaDataAndOwnerReferences struct {
	MetaDataName             string
	Status                   string
	Namespace                string
	OwnerReferenceName       string
	OwnerReferenceKind       string
	OwnerReferenceAPIVersion string
}

// Used for intermediate storage -- probably can be combined/merged with
// type Composition
type CompositionTreeNode struct {
	Level     int
	ChildKind string
	Children  []MetaDataAndOwnerReferences
}

// Used for intermediate storage -- probably can be merged with Composition
type Compositions struct {
	Kind            string
	Name            string
	Namespace       string
	Status          string
	CompositionTree *[]CompositionTreeNode
}

// Used to hold entire composition of all the Kinds
type ClusterCompositions struct {
	clusterCompositions []Compositions
	mux                 sync.Mutex
}

type Connection struct {
	Level           int
	Kind            string
	Name            string
	Namespace       string
	Owner           string
	RelationType	string
	RelationDetails string 
	OwnerKind 		string
	OwnerName    	string
	Peer           *Connection
}

type ConnectionOutput struct {
	Level           int
	Kind            string
	Name            string
	Namespace       string
	PeerKind		string
	PeerName		string
	PeerNamespace	string
	RelationType	string
	RelationDetails string
}

type KubeObjectCacheEntry struct {
	Namespace string
	Kind string
	Name string
	GVK schema.GroupVersionResource
}

var (

	ALLOWED_COMMANDS map[string]string

	USAGE_ANNOTATION string
	COMPOSITION_ANNOTATION string
	ANNOTATION_REL_ANNOTATION string
	LABEL_REL_ANNOTATION string
	SPECPROPERTY_REL_ANNOTATION string

	TotalClusterCompositions ClusterCompositions
	TotalClusterConnections []Connection

	KindPluralMap  map[string]string
	kindVersionMap map[string]string
	kindGroupMap map[string]string
	compositionMap map[string][]string
	relationshipMap map[string][]string
	crdcompositionMap map[string][]string

	kubeObjectListCache map[KubeObjectCacheEntry]interface{}
	kubeObjectCache map[KubeObjectCacheEntry]interface{}

	REPLICA_SET  string
	DEPLOYMENT   string
	POD          string
	SERVICE_ACCOUNT string
	CONFIG_MAP   string
	SERVICE      string
	SECRET       string
	PVCLAIM      string
	PV           string
	ETCD_CLUSTER string
	INGRESS      string
	STATEFULSET  string
	DAEMONSET    string
	RC           string
	PDB 		 string
	NAMESPACE    string

	relTypeLabel string
	relTypeSpecProperty string
	relTypeEnvvariable string
	relTypeAnnotation string
	relTypeOwnerReference string

	green, red, yellow, purple, cyan, reset string

	// Set to inputs given to connections
	OrigKind, OrigName, OrigNamespace string
	OrigLevel int
	OutputFormat string
	RelsToIgnore string

	NamespaceToSearch string
	OriginalInputNamespace string
	OriginalInputKind string
	OriginalInputInstance string
)

func init() {

	ALLOWED_COMMANDS = make(map[string]string,0)
	ALLOWED_COMMANDS["composition"] = "composition"
	ALLOWED_COMMANDS["connections"] = "connections"
	ALLOWED_COMMANDS["man"] = "man"
	ALLOWED_COMMANDS["networkmetrics"] = "networkmetrics"
	ALLOWED_COMMANDS["podmetrics"] = "podmetrics"

	TotalClusterCompositions = ClusterCompositions{}

	NamespaceToSearch = ""

	DEPLOYMENT = "Deployment"
	REPLICA_SET = "ReplicaSet"
	POD = "Pod"
	CONFIG_MAP = "ConfigMap"
	SERVICE = "Service"
	SECRET = "Secret"
	PVCLAIM = "PersistentVolumeClaim"
	PV = "PersistentVolume"
	ETCD_CLUSTER = "EtcdCluster"
	INGRESS = "Ingress"
	STATEFULSET = "StatefulSet"
	DAEMONSET = "DaemonSet"
	RC = "ReplicationController"
	PDB = "PodDisruptionBudget"
	SERVICE_ACCOUNT = "ServiceAccount"
	NAMESPACE = "Namespace"

	relTypeLabel = "label"
	relTypeSpecProperty = "specproperty"
	relTypeEnvvariable = "envvariable"
	relTypeAnnotation = "annotation"
	relTypeOwnerReference = "owner reference"

	green = "\033[32m"
	red   = "\033[31m"
	yellow = "\033[33m"
	purple = "\033[35m"
	cyan   = "\033[36m"
	reset = "\033[0m"

	KindPluralMap = make(map[string]string)
	// TODO: Change this to map[string][]string to support multiple versions
	kindVersionMap = make(map[string]string) 
	compositionMap = make(map[string][]string, 0)
	crdcompositionMap = make(map[string][]string, 0)
	kindGroupMap = make(map[string]string)
	relationshipMap = make(map[string][]string)

	kubeObjectListCache = make(map[KubeObjectCacheEntry]interface{})
	kubeObjectCache = make(map[KubeObjectCacheEntry]interface{})

	// set basic data types
	KindPluralMap[DEPLOYMENT] = "deployments"
	kindVersionMap[DEPLOYMENT] = "apis/apps/v1"
	kindGroupMap[DEPLOYMENT] = "apps"
	compositionMap[DEPLOYMENT] = []string{"ReplicaSet"}
	deploymentRelationships := make([]string,0)
	depRel := "owner reference, of:ReplicaSet, value:INSTANCE.name"
	deploymentRelationships = append(deploymentRelationships, depRel)
	relationshipMap[DEPLOYMENT] = deploymentRelationships

	KindPluralMap[REPLICA_SET] = "replicasets"
	kindVersionMap[REPLICA_SET] = "apis/apps/v1"
	kindGroupMap[REPLICA_SET] = "apps"
	compositionMap[REPLICA_SET] = []string{"Pod"}
	replicasetRelationships := make([]string,0)
	replicasetRel := "owner reference, of:Pod, value:INSTANCE.name"
	replicasetRelationships = append(replicasetRelationships, replicasetRel)
	relationshipMap[REPLICA_SET] = replicasetRelationships

	KindPluralMap[DAEMONSET] = "daemonsets"
	kindVersionMap[DAEMONSET] = "apis/apps/v1"
	kindGroupMap[DAEMONSET] = "apps"
	compositionMap[DAEMONSET] = []string{"Pod"}

	KindPluralMap[RC] = "replicationcontrollers"
	kindVersionMap[RC] = "api/v1"
	kindGroupMap[RC] = ""
	compositionMap[RC] = []string{"Pod"}

	KindPluralMap[PDB] = "poddisruptionbudgets"
	kindVersionMap[PDB] = "apis/policy/v1beta1"
	kindGroupMap[PDB] = "policy"
	compositionMap[PDB] = []string{}

	KindPluralMap[POD] = "pods"
	kindVersionMap[POD] = "api/v1"
	kindGroupMap[POD] = ""
	compositionMap[POD] = []string{}

	podRelationships := make([]string,0)
	podRel0 := "specproperty, on:INSTANCE.spec.env, value:Service.spec.metadata.name"
	podRel1 := "specproperty, on:INSTANCE.spec.volumes.persistentVolumeClaim.claimName, value:PersistentVolumeClaim.spec.metadata.name"
	podRel2 := "specproperty, on:INSTANCE.spec.serviceAccountName, value:ServiceAccount.metadata.name"
	podRel3 := "specproperty, on:INSTANCE.metadata.namespace, value:Namespace.metadata.name"	
	podRelationships = append(podRelationships, podRel0)
	podRelationships = append(podRelationships, podRel1)
	podRelationships = append(podRelationships, podRel2)
	podRelationships = append(podRelationships, podRel3)
	relationshipMap[POD] = podRelationships

	KindPluralMap[SERVICE_ACCOUNT] = "serviceaccounts"
	kindVersionMap[SERVICE_ACCOUNT] = "api/v1"
	kindGroupMap[SERVICE_ACCOUNT] = ""
	compositionMap[SERVICE_ACCOUNT] = []string{}

	KindPluralMap[NAMESPACE] = "namespaces"
	kindVersionMap[NAMESPACE] = "v1"
	kindGroupMap[NAMESPACE] = ""
	compositionMap[NAMESPACE] = []string{}

	KindPluralMap[SERVICE] = "services"
	kindVersionMap[SERVICE] = "api/v1"
	kindGroupMap[SERVICE] = ""
	compositionMap[SERVICE] = []string{}
	serviceRelationships := make([]string,0)
	serviceRel := "label, on:Pod, value:INSTANCE.spec.selector"
	serviceRelationships = append(serviceRelationships, serviceRel)
	relationshipMap[SERVICE] = serviceRelationships

	KindPluralMap[INGRESS] = "ingresses"
	kindVersionMap[INGRESS] = "networking.k8s.io/v1beta1"//"extensions/v1beta1"
	kindGroupMap[INGRESS] = "networking.k8s.io"
	compositionMap[INGRESS] = []string{}
	ingressRelationships := make([]string,0)
	ingressRel := "specproperty, on:INSTANCE.spec.rules.http.paths.backend.serviceName, value:Service.spec.metadata.name"
	ingressRelationships = append(ingressRelationships, ingressRel)
	relationshipMap[INGRESS] = ingressRelationships

	KindPluralMap[SECRET] = "secrets"
	kindVersionMap[SECRET] = "v1"
	kindGroupMap[SECRET] = ""
	compositionMap[SECRET] = []string{}

	KindPluralMap[PVCLAIM] = "persistentvolumeclaims"
	kindVersionMap[PVCLAIM] = "api/v1"
	kindGroupMap[PVCLAIM] = ""
	compositionMap[PVCLAIM] = []string{}
	pvcRelationships := make([]string,0)
	pvcRel := "specproperty, on:INSTANCE.spec.volumeName, value:PersistentVolume.metadata.name"
	pvcRelationships = append(pvcRelationships, pvcRel)
	relationshipMap[PVCLAIM] = pvcRelationships

	KindPluralMap[PV] = "persistentvolumes"
	kindVersionMap[PV] = "api/v1"
	kindGroupMap[PV] = ""
	compositionMap[PV] = []string{}

/*
	KindPluralMap[INGRESS] = "ingresses"
	kindVersionMap[INGRESS] = "apis/extensions/v1beta1"
	kindGroupMap[INGRESS] = "extensions"
	compositionMap[INGRESS] = []string{}
*/

	KindPluralMap[STATEFULSET] = "statefulsets"
	kindVersionMap[STATEFULSET] = "apis/apps/v1"
	kindGroupMap[STATEFULSET] = "apps"
	compositionMap[STATEFULSET] = []string{"Pod", "ReplicaSet"}
	ssetRelationships := make([]string,0)
	ssRel1 := "owner reference, of:ReplicaSet, value:INSTANCE.name"
	ssetRelationships = append(ssetRelationships, ssRel1)
	ssRel2 := "owner reference, of:Pod, value:INSTANCE.name"
	ssetRelationships = append(ssetRelationships, ssRel2)	
	relationshipMap[STATEFULSET] = ssetRelationships

	KindPluralMap[CONFIG_MAP] = "configmaps"
	kindVersionMap[CONFIG_MAP] = "api/v1"
	kindGroupMap[CONFIG_MAP] = ""
	compositionMap[CONFIG_MAP] = []string{}

	USAGE_ANNOTATION = "resource/usage"
	COMPOSITION_ANNOTATION = "resource/composition"
	ANNOTATION_REL_ANNOTATION = "resource/annotation-relationship"
	LABEL_REL_ANNOTATION = "resource/label-relationship"
	SPECPROPERTY_REL_ANNOTATION = "resource/specproperty-relationship"
}

func getKindAPIDetails(kind string) (string, string, string, string) {
	kindplural := KindPluralMap[kind]
	kindResourceApiVersion := kindVersionMap[kind]
	kindResourceGroup := kindGroupMap[kind]

	parts := strings.Split(kindResourceApiVersion, "/")
	kindAPI := parts[len(parts)-1]

	return kindplural, kindResourceApiVersion, kindAPI, kindResourceGroup
}