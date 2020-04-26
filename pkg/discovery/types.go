package discovery

import (
	"sync"
	"strings"
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

var (
	TotalClusterCompositions ClusterCompositions

	KindPluralMap  map[string]string
	kindVersionMap map[string]string
	kindGroupMap map[string]string
	compositionMap map[string][]string
	relationshipMap map[string][]string

	REPLICA_SET  string
	DEPLOYMENT   string
	POD          string
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
)

func init() {
	TotalClusterCompositions = ClusterCompositions{}

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

	KindPluralMap = make(map[string]string)
	// TODO: Change this to map[string][]string to support multiple versions
	kindVersionMap = make(map[string]string) 
	compositionMap = make(map[string][]string, 0)
	kindGroupMap = make(map[string]string)
	relationshipMap = make(map[string][]string)

	// set basic data types
	KindPluralMap[DEPLOYMENT] = "deployments"
	kindVersionMap[DEPLOYMENT] = "apis/apps/v1"
	kindGroupMap[DEPLOYMENT] = "apps"
	compositionMap[DEPLOYMENT] = []string{"ReplicaSet"}

	KindPluralMap[REPLICA_SET] = "replicasets"
	kindVersionMap[REPLICA_SET] = "apis/extensions/v1beta1"
	kindGroupMap[REPLICA_SET] = "extensions"
	compositionMap[REPLICA_SET] = []string{"Pod"}

	KindPluralMap[DAEMONSET] = "daemonsets"
	kindVersionMap[DAEMONSET] = "apis/extensions/v1beta1"
	kindGroupMap[DAEMONSET] = "extensions"
	compositionMap[DAEMONSET] = []string{"Pod"}

	KindPluralMap[DAEMONSET] = "daemonsets"
	kindVersionMap[DAEMONSET] = "apis/apps/v1" //Bug: This will overwrite assignment in 104.
	kindGroupMap[DAEMONSET] = "apps"
	compositionMap[DAEMONSET] = []string{"Pod"}

	KindPluralMap[RC] = "replicationcontrollers"
	kindVersionMap[RC] = "api/v1"
	kindGroupMap[RC] = ""
	compositionMap[RC] = []string{"Pod"}

	KindPluralMap[POD] = "pods"
	kindVersionMap[POD] = "api/v1"
	kindGroupMap[POD] = ""
	compositionMap[POD] = []string{}
	relationshipMap[POD] = []string{}

	KindPluralMap[PDB] = "poddisruptionbudgets"
	kindVersionMap[PDB] = "apis/policy/v1beta1"
	kindGroupMap[PDB] = "policy"
	compositionMap[PDB] = []string{}

	KindPluralMap[SERVICE] = "services"
	kindVersionMap[SERVICE] = "api/v1"
	kindGroupMap[SERVICE] = ""
	compositionMap[SERVICE] = []string{}
	serviceRelationships := make([]string,0)
	serviceRel := "label, on:Pod;Deployment, value:instance.spec.selector"
	serviceRelationships = append(serviceRelationships, serviceRel)
	relationshipMap[SERVICE] = serviceRelationships

	KindPluralMap[SECRET] = "secrets"
	kindVersionMap[SECRET] = "api/v1"
	kindGroupMap[SECRET] = ""
	compositionMap[SECRET] = []string{}

	KindPluralMap[PVCLAIM] = "persistentvolumeclaims"
	kindVersionMap[PVCLAIM] = "api/v1"
	kindGroupMap[PVCLAIM] = ""
	compositionMap[PVCLAIM] = []string{}

	KindPluralMap[PV] = "persistentvolumes"
	kindVersionMap[PV] = "api/v1"
	kindGroupMap[PV] = ""
	compositionMap[PV] = []string{}

	KindPluralMap[INGRESS] = "ingresses"
	kindVersionMap[INGRESS] = "apis/extensions/v1beta1"
	kindGroupMap[INGRESS] = "extensions"
	compositionMap[INGRESS] = []string{}

	KindPluralMap[STATEFULSET] = "statefulsets"
	kindVersionMap[STATEFULSET] = "apis/apps/v1"
	kindGroupMap[STATEFULSET] = "apps"
	compositionMap[STATEFULSET] = []string{"Pod", "ReplicaSet"}

	KindPluralMap[CONFIG_MAP] = "configmaps"
	kindVersionMap[CONFIG_MAP] = "api/v1"
	kindGroupMap[CONFIG_MAP] = ""
	compositionMap[CONFIG_MAP] = []string{}
}

func getKindAPIDetails(kind string) (string, string, string, string) {
	kindplural := KindPluralMap[kind]
	kindResourceApiVersion := kindVersionMap[kind]
	kindResourceGroup := kindGroupMap[kind]

	parts := strings.Split(kindResourceApiVersion, "/")
	kindAPI := parts[len(parts)-1]

	return kindplural, kindResourceApiVersion, kindAPI, kindResourceGroup
}