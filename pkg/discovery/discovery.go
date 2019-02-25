package discovery

import (
	"crypto/tls"
	cert "crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	serviceHost    string
	servicePort    string
	Namespace      string
	httpMethod     string
	etcdServiceURL string

	KindPluralMap  map[string]string
	kindVersionMap map[string]string
	compositionMap map[string][]string

	REPLICA_SET  string
	DEPLOYMENT   string
	POD          string
	CONFIG_MAP   string
	SERVICE      string
	SECRET       string
	PVCLAIM      string
	PV           string
	ETCD_CLUSTER string
)

var (
	masterURL   string
	kubeconfig  string
	etcdservers string
)

func init() {

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&etcdservers, "etcd-servers", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	flag.Parse()
	serviceHost = os.Getenv("KUBERNETES_SERVICE_HOST")
	servicePort = os.Getenv("KUBERNETES_SERVICE_PORT")
	Namespace = "default"
	httpMethod = http.MethodGet

	etcdServiceURL = "http://localhost:2379"

	DEPLOYMENT = "Deployment"
	REPLICA_SET = "ReplicaSet"
	POD = "Pod"
	CONFIG_MAP = "ConfigMap"
	SERVICE = "Service"
	SECRET = "Secret"
	PVCLAIM = "PersistentVolumeClaim"
	PV = "PersistentVolume"
	ETCD_CLUSTER = "EtcdCluster"

	KindPluralMap = make(map[string]string)
	kindVersionMap = make(map[string]string)
	compositionMap = make(map[string][]string, 0)

	readKindCompositionFile()

	// set basic data types
	KindPluralMap[DEPLOYMENT] = "deployments"
	kindVersionMap[DEPLOYMENT] = "apis/apps/v1"
	compositionMap[DEPLOYMENT] = []string{"ReplicaSet"}

	KindPluralMap[REPLICA_SET] = "replicasets"
	kindVersionMap[REPLICA_SET] = "apis/extensions/v1beta1"
	compositionMap[REPLICA_SET] = []string{"Pod"}

	KindPluralMap[POD] = "pods"
	kindVersionMap[POD] = "api/v1"
	compositionMap[POD] = []string{}

	KindPluralMap[SERVICE] = "services"
	kindVersionMap[SERVICE] = "api/v1"
	compositionMap[SERVICE] = []string{}

	KindPluralMap[SECRET] = "secrets"
	kindVersionMap[SECRET] = "api/v1"
	compositionMap[SECRET] = []string{}

	KindPluralMap[PVCLAIM] = "persistentvolumeclaims"
	kindVersionMap[PVCLAIM] = "api/v1"
	compositionMap[PVCLAIM] = []string{}

	KindPluralMap[PV] = "persistentvolumes"
	kindVersionMap[PV] = "api/v1/persistentvolumes"
	compositionMap[PV] = []string{}
}

func BuildCompositionTree() {
	for {
		readKindCompositionFile()
		resourceKindList := getResourceKinds()
		resourceInCluster := []MetaDataAndOwnerReferences{}
		for _, resourceKind := range resourceKindList {
			topLevelMetaDataOwnerRefList := getResourceNames(resourceKind)
			//fmt.Printf("TopLevelMetaDataOwnerRefList:%v\n", topLevelMetaDataOwnerRefList)
			for _, topLevelObject := range topLevelMetaDataOwnerRefList {
				resourceName := topLevelObject.MetaDataName

				level := 1
				compositionTree := []CompositionTreeNode{}
				buildCompositions(resourceKind, resourceName, level, &compositionTree)
				//fmt.Printf("CompositionTree:%v\n", compositionTree)
				TotalClusterCompositions.storeCompositions(topLevelObject, resourceKind, resourceName, &compositionTree)
			}
			for _, resource := range topLevelMetaDataOwnerRefList {
				present := false
				for _, res := range resourceInCluster {
					if res.MetaDataName == resource.MetaDataName {
						present = true
					}
				}
				if !present {
					resourceInCluster = append(resourceInCluster, resource)
				}
			}
		}

		TotalClusterCompositions.purgeCompositionOfDeletedItems(resourceInCluster)

		time.Sleep(time.Second * 10)
	}
}

func (cp *ClusterCompositions) checkIfProvenanceNeeded(resourceKind, resourceName string) bool {
	cp.mux.Lock()
	defer cp.mux.Unlock()
	for _, compositionItem := range cp.clusterCompositions {
		kind := compositionItem.Kind
		name := compositionItem.Name
		if resourceKind == kind && resourceName == name {
			return false
		}
	}
	return true
}

func readKindCompositionFile() {
	// read from the opt file
	filePath, ok := os.LookupEnv("KIND_COMPOSITION_FILE")
	if ok {
		yamlFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading file:%s", err)
		}

		compositionsList := make([]composition, 0)
		err = yaml.Unmarshal(yamlFile, &compositionsList)

		for _, compositionObj := range compositionsList {
			kind := compositionObj.Kind
			endpoint := compositionObj.Endpoint
			composition := compositionObj.Composition
			plural := compositionObj.Plural
			//fmt.Printf("Kind:%s, Plural: %s Endpoint:%s, Composition:%s\n", kind, plural, endpoint, composition)

			KindPluralMap[kind] = plural
			kindVersionMap[kind] = endpoint
			compositionMap[kind] = composition
		}
	} else {
		// Populate the Kind maps by querying CRDs from ETCD and querying KAPI for details of each CRD
		crdListString, err := queryETCD("/operators")
		if err == nil && crdListString != "" {
			crdNameList := getCRDNames(crdListString)

			for _, crdName := range crdNameList {
				crdDetailsString, err := queryETCD("/" + crdName)
				if err != nil {
					fmt.Printf("Error: %s\n", err.Error())
					return
				}
				kind, plural, endpoint, composition := getCRDDetails(crdDetailsString)

				KindPluralMap[kind] = plural
				kindVersionMap[kind] = endpoint
				compositionMap[kind] = composition
			}
		}
	}
	//printMaps()
}

func getResourceKinds() []string {
	resourceKindSlice := make([]string, 0)
	for key, _ := range compositionMap {
		resourceKindSlice = append(resourceKindSlice, key)
	}
	return resourceKindSlice
}

func getResourceNames(resourceKind string) []MetaDataAndOwnerReferences {
	resourceApiVersion := kindVersionMap[resourceKind]
	resourceKindPlural := KindPluralMap[resourceKind]
	content := queryAPIServer(resourceApiVersion, resourceKindPlural)
	metaDataAndOwnerReferenceList := parseMetaData(content)
	return metaDataAndOwnerReferenceList
}

func processed(processedList *[]CompositionTreeNode, nodeToCheck CompositionTreeNode) bool {
	//fmt.Printf("ProcessedList:%v\n", processedList)
	//fmt.Printf("NodeToCheck:%v\n", nodeToCheck)
	var result bool = false
	for _, compositionTreeNode1 := range *processedList {
		if compositionTreeNode1.Level == nodeToCheck.Level && compositionTreeNode1.ChildKind == nodeToCheck.ChildKind {
			result = true
		}
	}
	return result
}

func getComposition(kind, name, status string, level int, compositionTree *[]CompositionTreeNode,
	processedList *[]CompositionTreeNode) Composition {
	//var compositionsString string
	//fmt.Printf("-- Kind: %s Name: %s\n", kind, name)
	//compositionsString = "Kind: " + kind + " Name:" + name + " Composition:\n"
	parentComposition := Composition{}
	parentComposition.Level = level
	parentComposition.Kind = kind
	parentComposition.Name = name
	parentComposition.Status = status
	parentComposition.Children = []Composition{}

	//fmt.Printf("CompositionTree:%v\n", compositionTree)

	for _, compositionTreeNode := range *compositionTree {
		if processed(processedList, compositionTreeNode) {
			continue
		}
		level := compositionTreeNode.Level
		childKind := compositionTreeNode.ChildKind
		metaDataAndOwnerReferences := compositionTreeNode.Children

		for _, metaDataNode := range metaDataAndOwnerReferences {
			//compositionsString = compositionsString + " " + string(level) + " " + childKind + " " + childName + "\n"
			childName := metaDataNode.MetaDataName
			childStatus := metaDataNode.Status
			trimmedTree := []CompositionTreeNode{}
			for _, compositionTreeNode1 := range *compositionTree {
				if compositionTreeNode1.Level != level && compositionTreeNode1.ChildKind != childKind {
					trimmedTree = append(trimmedTree, compositionTreeNode1)
				}
			}
			*processedList = append(*processedList, compositionTreeNode)
			child := getComposition(childKind, childName, childStatus, level, &trimmedTree, processedList)
			parentComposition.Children = append(parentComposition.Children, child)
			compositionTree = &[]CompositionTreeNode{}
		}
	}
	return parentComposition
}

func (cp *ClusterCompositions) GetCompositions(resourceKind, resourceName string) string {
	cp.mux.Lock()
	defer cp.mux.Unlock()
	var compositionBytes []byte
	var compositionString string
	compositions := []Composition{}

	resourceKindPlural := KindPluralMap[resourceKind]

	//fmt.Println("Compositions of different Kinds in this Cluster")
	//fmt.Printf("Kind:%s, Name:%s\n", resourceKindPlural, resourceName)
	for _, compositionItem := range cp.clusterCompositions {
		kind := strings.ToLower(compositionItem.Kind)
		name := strings.ToLower(compositionItem.Name)
		status := compositionItem.Status
		compositionTree := compositionItem.CompositionTree
		resourceKindPlural := strings.ToLower(resourceKindPlural)
		//TODO(devdattakulkarni): Make route registration and compositions keyed info
		//to use same kind name (plural). Currently Compositions info is keyed on
		//singular kind names. For now, trimming the 's' at the end
		//resourceKind = strings.TrimSuffix(resourceKind, "s")
		var resourceKind string
		for key, value := range KindPluralMap {
			if strings.ToLower(value) == strings.ToLower(resourceKindPlural) {
				resourceKind = strings.ToLower(key)
				break
			}
		}
		resourceName := strings.ToLower(resourceName)
		//fmt.Printf("Kind:%s, Kind:%s, Name:%s, Name:%s\n", kind, resourceKind, name, resourceName)
		if resourceName == "*" {
			if resourceKind == kind {
				processedList := []CompositionTreeNode{}
				level := 1
				composition := getComposition(kind, name, status, level, compositionTree, &processedList)
				compositions = append(compositions, composition)
			}
		} else if resourceKind == kind && resourceName == name {
			processedList := []CompositionTreeNode{}
			level := 1
			composition := getComposition(kind, name, status, level, compositionTree, &processedList)
			compositions = append(compositions, composition)
		}
	}

	compositionBytes, err := json.Marshal(compositions)
	if err != nil {
		fmt.Println(err)
	}
	compositionString = string(compositionBytes)
	return compositionString
}

func (cp *ClusterCompositions) purgeCompositionOfDeletedItems(topLevelMetaDataOwnerRefList []MetaDataAndOwnerReferences) {
	presentList := []Compositions{}
	//fmt.Println("ClusterCompositions:%v\n", cp.clusterCompositions)
	//fmt.Println("ToplevelMetaDataOwnerList:%v\n", topLevelMetaDataOwnerRefList)
	for _, compositionItem := range cp.clusterCompositions {
		for _, topLevelObject := range topLevelMetaDataOwnerRefList {
			resourceName := topLevelObject.MetaDataName
			//fmt.Printf("ResourceName:%s, prov.Name:%s\n", resourceName, prov.Name)
			if resourceName == compositionItem.Name {
				presentList = append(presentList, compositionItem)
			}
		}
	}
	//fmt.Printf("Updated Cluster Prov List:%v\n", presentList)
	cp.clusterCompositions = presentList
}

// This stores Compositions information in memory. The compositions information will be lost
// when this Pod is deleted.
func (cp *ClusterCompositions) storeCompositions(topLevelObject MetaDataAndOwnerReferences,
	resourceKind string, resourceName string,
	compositionTree *[]CompositionTreeNode) {
	cp.mux.Lock()
	defer cp.mux.Unlock()
	compositions := Compositions{
		Kind:            resourceKind,
		Name:            resourceName,
		Status:          topLevelObject.Status,
		CompositionTree: compositionTree,
	}
	present := false
	// If prov already exists then replace status and composition Tree
	//fmt.Printf("00 CP:%v\n", cp.clusterCompositions)
	for i, comp := range cp.clusterCompositions {
		if comp.Kind == compositions.Kind && comp.Name == compositions.Name {
			present = true
			p := &comp
			//fmt.Printf("CompositionTree:%v\n", compositionTree)
			p.CompositionTree = compositionTree
			p.Status = topLevelObject.Status
			cp.clusterCompositions[i] = *p
			//fmt.Printf("11 CP:%v\n", cp.clusterCompositions)
		}
	}
	if !present {
		cp.clusterCompositions = append(cp.clusterCompositions, compositions)
		//fmt.Printf("22 CP:%v\n", cp.clusterCompositions)
	}
	//fmt.Println("Exiting storeCompositions")
	//fmt.Printf("ClusterCompositions:%v\n", cp.clusterCompositions)
}

func buildCompositions(parentResourceKind string, parentResourceName string, level int,
	compositionTree *[]CompositionTreeNode) {
	childResourceKindList, present := compositionMap[parentResourceKind]
	if present {
		level = level + 1

		for _, childResourceKind := range childResourceKindList {
			childKindPlural := KindPluralMap[childResourceKind]
			childResourceApiVersion := kindVersionMap[childResourceKind]
			var content []byte
			var metaDataAndOwnerReferenceList []MetaDataAndOwnerReferences
			content = queryAPIServer(childResourceApiVersion, childKindPlural)
			metaDataAndOwnerReferenceList = parseMetaData(content)

			childrenList := filterChildren(&metaDataAndOwnerReferenceList, parentResourceName)
			compTreeNode := CompositionTreeNode{
				Level:     level,
				ChildKind: childResourceKind,
				Children:  childrenList,
			}

			*compositionTree = append(*compositionTree, compTreeNode)

			for _, metaDataRef := range childrenList {
				resourceName := metaDataRef.MetaDataName
				resourceKind := childResourceKind
				buildCompositions(resourceKind, resourceName, level, compositionTree)
			}
		}
	} else {
		return
	}
}

func queryAPIServer(resourceApiVersion, resourcePlural string) []byte {
	//fmt.Println("Entering queryAPIServer")
	var url1 string
	if !strings.Contains(resourceApiVersion, resourcePlural) {
		url1 = fmt.Sprintf("https://%s:%s/%s/namespaces/%s/%s", serviceHost, servicePort, resourceApiVersion, Namespace, resourcePlural)
	} else {
		url1 = fmt.Sprintf("https://%s:%s/%s", serviceHost, servicePort, resourceApiVersion)
	}
	//fmt.Printf("Url:%s\n",url1)
	caToken := getToken()
	caCertPool := getCACert()
	u, err := url.Parse(url1)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(httpMethod, u.String(), nil)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(caToken)))
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("sending request failed: %s", err.Error())
		fmt.Println(err)
	}
	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	//fmt.Println(resp.Status)
	//fmt.Println(string(resp_body))
	//fmt.Println("Exiting queryAPIServer")
	return resp_body
}

//Ref:https://www.sohamkamani.com/blog/2017/10/18/parsing-json-in-golang/#unstructured-data
func parseMetaData(content []byte) []MetaDataAndOwnerReferences {
	//fmt.Println("Entering parseMetaData")
	var result map[string]interface{}
	json.Unmarshal([]byte(content), &result)
	// We need to parse following from the result
	// metadata.name
	// metadata.ownerReferences.name
	// metadata.ownerReferences.kind
	// metadata.ownerReferences.apiVersion
	metaDataSlice := []MetaDataAndOwnerReferences{}
	items, ok := result["items"].([]interface{})

	if ok {
		for _, item := range items {
			//fmt.Println("=======================")
			itemConverted := item.(map[string]interface{})
			var metadataProcessed, statusProcessed bool
			metaDataRef := MetaDataAndOwnerReferences{}
			statusKeyExists := false
			for key, _ := range itemConverted {
				if key == "status" {
					statusKeyExists = true
				}
			}
			for key, value := range itemConverted {
				if key == "metadata" {
					//fmt.Println("----")
					//fmt.Println(key, value.(interface{}))
					metadataMap := value.(map[string]interface{})
					for mkey, mvalue := range metadataMap {
						//fmt.Printf("%v ==> %v\n", mkey, mvalue.(interface{}))
						if mkey == "ownerReferences" {
							ownerReferencesList := mvalue.([]interface{})
							for _, ownerReference := range ownerReferencesList {
								ownerReferenceMap := ownerReference.(map[string]interface{})
								for okey, ovalue := range ownerReferenceMap {
									//fmt.Printf("%v --> %v\n", okey, ovalue)
									if okey == "name" {
										metaDataRef.OwnerReferenceName = ovalue.(string)
									}
									if okey == "kind" {
										metaDataRef.OwnerReferenceKind = ovalue.(string)
									}
									if okey == "apiVersion" {
										metaDataRef.OwnerReferenceAPIVersion = ovalue.(string)
									}
								}
							}
						}
						if mkey == "name" {
							metaDataRef.MetaDataName = mvalue.(string)
						}
					}
					metadataProcessed = true
				}
				if key == "status" {
					statusMap := value.(map[string]interface{})
					var replicas, readyReplicas, availableReplicas float64
					for skey, svalue := range statusMap {
						if skey == "phase" {
							metaDataRef.Status = svalue.(string)
							//fmt.Printf("Status:%s\n", metaDataRef.Status)
						}
						if skey == "replicas" {
							replicas = svalue.(float64)
						}
						if skey == "readyReplicas" {
							readyReplicas = svalue.(float64)
						}
						if skey == "availableReplicas" {
							availableReplicas = svalue.(float64)
						}
					}
					// Trying to be completely sure that we can set READY status
					if replicas > 0 {
						if replicas == availableReplicas && replicas == readyReplicas {
							metaDataRef.Status = "Ready"
						}
					}
					statusProcessed = true
				}
				if statusKeyExists {
					if metadataProcessed && statusProcessed {
						metaDataSlice = append(metaDataSlice, metaDataRef)
					}
				} else if metadataProcessed {
					metaDataSlice = append(metaDataSlice, metaDataRef)
				}
			}
		}
	}
	//fmt.Println("Exiting parseMetaData")
	//fmt.Printf("Metadata slice:%v\n", metaDataSlice)
	return metaDataSlice
}

func filterChildren(metaDataSlice *[]MetaDataAndOwnerReferences, parentResourceName string) []MetaDataAndOwnerReferences {
	metaDataSliceToReturn := []MetaDataAndOwnerReferences{}
	for _, metaDataRef := range *metaDataSlice {
		if metaDataRef.OwnerReferenceName == parentResourceName {
			// Prevent duplicates
			present := false
			for _, node := range metaDataSliceToReturn {
				if node.MetaDataName == metaDataRef.MetaDataName {
					present = true
				}
			}
			if !present {
				metaDataSliceToReturn = append(metaDataSliceToReturn, metaDataRef)
			}
		}
	}
	return metaDataSliceToReturn
}

// Ref:https://stackoverflow.com/questions/30690186/how-do-i-access-the-kubernetes-api-from-within-a-pod-container
func getToken() []byte {
	caToken, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		panic(err) // cannot find token file
	}
	//fmt.Printf("Token:%s", caToken)
	return caToken
}

// Ref:https://stackoverflow.com/questions/30690186/how-do-i-access-the-kubernetes-api-from-within-a-pod-container
func getCACert() *cert.CertPool {
	caCertPool := cert.NewCertPool()
	caCert, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		panic(err) // Can't find cert file
	}
	//fmt.Printf("CaCert:%s",caCert)
	caCertPool.AppendCertsFromPEM(caCert)
	return caCertPool
}
