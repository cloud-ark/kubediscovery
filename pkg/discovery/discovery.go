package discovery

import (
	"crypto/tls"
	"context"
	cert "crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"strconv"
	"gopkg.in/yaml.v2"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
    apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	serviceHost    string
	servicePort    string

	masterURL   string
	etcdservers string
	caToken		[]byte
	caCertPool	*cert.CertPool
)

func init() {

	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&etcdservers, "etcd-servers", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Parse()

	serviceHost = os.Getenv("KUBERNETES_SERVICE_HOST")
	servicePort = os.Getenv("KUBERNETES_SERVICE_PORT")

	cfg, err = buildConfig()
	if err != nil {
		panic(err.Error())
	}
}

func BuildCompositionTree(namespace string) {
	var namespaces []string
	namespaces = append(namespaces, namespace)

	err := readKindCompositionFile("")
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}
	resourceKindList := getResourceKinds()

	resourceInCluster := []MetaDataAndOwnerReferences{}
	for _, resourceKind := range resourceKindList {
		for _, namespace := range namespaces {
			topLevelMetaDataOwnerRefList := getTopLevelResourceMetaData(resourceKind, namespace)
			for _, topLevelObject := range topLevelMetaDataOwnerRefList {
				resourceName := topLevelObject.MetaDataName
				namespace := topLevelObject.Namespace
				level := 1
				compositionTree := []CompositionTreeNode{}
				buildCompositions(resourceKind, resourceName, namespace, level, &compositionTree)
				TotalClusterCompositions.storeCompositions(topLevelObject, resourceKind, resourceName, namespace, &compositionTree)
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
	}
	TotalClusterCompositions.purgeCompositionOfDeletedItems(resourceInCluster)
}

func readKindCompositionFile(inputKind string) error {
	filePath, ok := os.LookupEnv("KIND_COMPOSITION_FILE")
	if ok {
		yamlFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}

		compositionsList := make([]composition, 0)
		err = yaml.Unmarshal(yamlFile, &compositionsList)
		if err != nil {
			return err
		}
		for _, compositionObj := range compositionsList {
			kind := compositionObj.Kind
			endpoint := compositionObj.Endpoint
			composition := compositionObj.Composition
			plural := compositionObj.Plural

			KindPluralMap[kind] = plural
			kindVersionMap[kind] = endpoint
			compositionMap[kind] = composition
		}
	} else {
		crdClient, err1 := apiextensionsclientset.NewForConfig(cfg)
		if err1 != nil {
			panic(err1.Error())
		}
		crdList, err := crdClient.CustomResourceDefinitions().List(context.TODO(),
																   metav1.ListOptions{})
		if err != nil {
			fmt.Errorf("Error:%s\n", err)
			return err
		}
		for _, crd := range crdList.Items {
			crdName := crd.ObjectMeta.Name
			//fmt.Printf("CRD NAME:%s\n", crdName)
			crdObj, err := crdClient.CustomResourceDefinitions().Get(context.TODO(),
													     			 crdName, 
																	 metav1.GetOptions{})
			if err != nil {
				fmt.Errorf("Error:%s\n", err)
				panic(err)
				return err
			}
			//fmt.Printf("InputKind:%s, thisKind:%s\n", inputKind, crdObj.Spec.Names.Kind)
			/*if inputKind != "" {
				if inputKind == crdObj.Spec.Names.Kind {
					parseCRDAnnotions(crdObj)
					break
				}
			} else {
				parseCRDAnnotions(crdObj)
			}*/
			parseCRDAnnotions(crdObj)
		}
	}
	return nil
}

func parseCRDAnnotions(crdObj *apiextensionsv1beta1.CustomResourceDefinition) {

	//fmt.Printf("Inside parseCRDAnnotions\n")
	group := crdObj.Spec.Group
	version := crdObj.Spec.Version
	endpoint := "apis/" + group + "/" + version
	kind := crdObj.Spec.Names.Kind
	plural := crdObj.Spec.Names.Plural
	KindPluralMap[kind] = plural
	kindVersionMap[kind] = endpoint
	kindGroupMap[kind] = group

	objectMeta := crdObj.ObjectMeta
	annotations := objectMeta.GetAnnotations()
	//fmt.Printf("%v\n", annotations)
	//fmt.Printf("&&&&\n")
	compositionAnnotation := annotations[COMPOSITION_ANNOTATION]
	if compositionAnnotation != "" {
		compositionMap[kind] = strings.Split(compositionAnnotation, ",")
	}
	//fmt.Printf("=====\n")
	allRels := getAllRelationships(annotations)
	//printRels(allRels)
	relationshipMap[kind] = allRels
}

func getAllRelationships(annotations map[string]string) []string {
	annotationRels := parseRels(annotations, ANNOTATION_REL_ANNOTATION, "annotation")
	labelRels := parseRels(annotations, LABEL_REL_ANNOTATION, "label")
	specPropertyRels := parseRels(annotations, SPECPROPERTY_REL_ANNOTATION, "specproperty")
		//	fmt.Printf(annotationRels)
	//fmt.Printf("A\n")
	//printRels(annotationRels)
	//fmt.Printf("B\n")
	//printRels(labelRels)
	//fmt.Printf("C\n")
	//printRels(specPropertyRels)
	allRels := make([]string,0)
	allRels = mergeRels(allRels, specPropertyRels)
	allRels = mergeRels(allRels, labelRels)
	allRels = mergeRels(allRels, annotationRels)
	return allRels
} 

func printRels(rels []string) {
	for _, rel := range rels {
		fmt.Printf("%s\n", rel)				
	}
	//fmt.Printf("----\n")
}


func mergeRels(allRels, relsToAdd []string) []string {
	for _, item := range relsToAdd {
		allRels = append(allRels, item)
	}
	return allRels
}

func parseRels(annotations map[string]string, rel, relType string) []string {
	rels := make([]string,0)
	count := 1
	for key, value := range annotations {
		extendedKey := rel+strconv.Itoa(count)
		//fmt.Printf("ExtendedKey:%s\n", extendedKey)
		//fmt.Printf("key:%s, rel:%s, extendedKey:%s\n", key, rel, extendedKey)
		if key == rel || key == extendedKey {
			//fmt.Printf("(%s, %s)\n", key, value)
			newRelValue := relType + ", " + value
			rels = append(rels, newRelValue)
			if key == extendedKey {
				count = count + 1
			}
			//fmt.Printf("%s,", relValue)
		}
	}
	//print("^^^^\n")
	//printRels(rels)
	return rels
}

func getResourceKinds() []string {
	resourceKindSlice := make([]string, 0)
	for key, _ := range compositionMap {
		resourceKindSlice = append(resourceKindSlice, key)
	}
	return resourceKindSlice
}

func getResourceMetaData(resourceKindPlural, resourceGroup, resourceApiVersion,
						 namespace string) []MetaDataAndOwnerReferences {

	metaDataAndOwnerReferenceList := []MetaDataAndOwnerReferences{}

	//fmt.Printf("Res:%s, Group:%s, Version:%s\n", resourceKindPlural, resourceGroup, apiPart)

	if resourceKindPlural == "" || resourceApiVersion == "" {
		return metaDataAndOwnerReferenceList
	}

	dynamicClient, err := getDynamicClient()
	if err != nil {
		return metaDataAndOwnerReferenceList
	}

	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}

	list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return metaDataAndOwnerReferenceList
	}

	for _, unstructuredObj := range list.Items {
		metaDataRef := MetaDataAndOwnerReferences{}
		ownerReferences := unstructuredObj.GetOwnerReferences()
		for _, ownerReference := range ownerReferences {
			metaDataRef.OwnerReferenceKind = ownerReference.Kind
			metaDataRef.OwnerReferenceName = ownerReference.Name
			metaDataRef.OwnerReferenceAPIVersion = ownerReference.APIVersion
		}
		metaDataRef.Namespace = unstructuredObj.GetNamespace()
		metaDataRef.MetaDataName = unstructuredObj.GetName()

		content := unstructuredObj.UnstructuredContent()
		phase, found, _ := unstructured.NestedString(content, "status", "phase")
		if found {
			metaDataRef.Status = phase
		}

		metaDataAndOwnerReferenceList = append(metaDataAndOwnerReferenceList, metaDataRef)
	}

	return metaDataAndOwnerReferenceList
}

func getTopLevelResourceMetaData(resourceKind, namespace string) []MetaDataAndOwnerReferences {
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(resourceKind)

	metaDataAndOwnerReferenceList := getResourceMetaData(resourceKindPlural,
														 resourceGroup,
														 resourceApiVersion,
														 namespace)
	return metaDataAndOwnerReferenceList
}

func processed(processedList *[]CompositionTreeNode, nodeToCheck CompositionTreeNode) bool {
	var result bool = false
	for _, compositionTreeNode1 := range *processedList {
		if compositionTreeNode1.Level == nodeToCheck.Level && compositionTreeNode1.ChildKind == nodeToCheck.ChildKind {
			result = true
		}
	}
	return result
}

func getComposition(kind, name, namespace, status string, level int, compositionTree *[]CompositionTreeNode,
	processedList *[]CompositionTreeNode) Composition {
	parentComposition := Composition{}
	parentComposition.Level = level
	parentComposition.Kind = kind
	parentComposition.Name = name
	parentComposition.Namespace = namespace
	parentComposition.Status = status
	parentComposition.Children = []Composition{}

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
			childNamespace := metaDataNode.Namespace

			// childNamespace := metaDataNode.MetaDataNamespace
			childStatus := metaDataNode.Status
			trimmedTree := []CompositionTreeNode{}
			for _, compositionTreeNode1 := range *compositionTree {
				if compositionTreeNode1.Level != level && compositionTreeNode1.ChildKind != childKind {
					trimmedTree = append(trimmedTree, compositionTreeNode1)
				}
			}
			*processedList = append(*processedList, compositionTreeNode)
			child := getComposition(childKind, childName, childNamespace, childStatus, level, &trimmedTree, processedList)
			parentComposition.Children = append(parentComposition.Children, child)
			compositionTree = &[]CompositionTreeNode{}
		}
	}
	return parentComposition
}

func (cp *ClusterCompositions) GetCompositions(resourceKind, resourceName, namespace string) string {
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
		nmspace := strings.ToLower(compositionItem.Namespace)
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

		//TODO(devdattakulkarni): Below case statements look suspect as actions of two
		//cases are similar. Can these be combined?
		switch {
		case namespace != nmspace:
			break
		case resourceName == "*" && resourceKind == kind && namespace == nmspace:
			processedList := []CompositionTreeNode{}
			level := 1
			composition := getComposition(kind, name, namespace, status, level, compositionTree, &processedList)
			compositions = append(compositions, composition)
			break
		case resourceName == name && resourceKind == kind && namespace == nmspace:
			processedList := []CompositionTreeNode{}
			level := 1
			composition := getComposition(kind, name, namespace, status, level, compositionTree, &processedList)
			compositions = append(compositions, composition)
			break
		}
	}

	compositionBytes, err := json.Marshal(compositions)
	if err != nil {
		fmt.Println(err.Error())
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
			if resourceName == compositionItem.Name {
				presentList = append(presentList, compositionItem)
			}
		}
	}
	cp.clusterCompositions = presentList
}

// This stores Compositions information in memory. The compositions information will be lost
// when this Pod is deleted.
func (cp *ClusterCompositions) storeCompositions(topLevelObject MetaDataAndOwnerReferences,
	resourceKind, resourceName, namespace string,
	compositionTree *[]CompositionTreeNode) {
	cp.mux.Lock()
	defer cp.mux.Unlock()
	compositions := Compositions{
		Kind:            resourceKind,
		Name:            resourceName,
		Namespace:       namespace,
		Status:          topLevelObject.Status,
		CompositionTree: compositionTree,
	}
	present := false

	for i, comp := range cp.clusterCompositions {
		if comp.Kind == compositions.Kind && comp.Name == compositions.Name && comp.Namespace == compositions.Namespace {
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
	}
}

func buildCompositions(parentResourceKind string, parentResourceName string, parentNamespace string, level int,
	compositionTree *[]CompositionTreeNode) {
	childResourceKindList, present := compositionMap[parentResourceKind]
	if present {
		level = level + 1

		for _, childResourceKind := range childResourceKindList {
			childResourceKind = strings.TrimSpace(childResourceKind)
			childKindPlural, _, childResourceApiVersion, childResourceGroup := getKindAPIDetails(childResourceKind)

			//var content []byte
			var metaDataAndOwnerReferenceList []MetaDataAndOwnerReferences

			metaDataAndOwnerReferenceList = getResourceMetaData(childKindPlural,
																childResourceGroup,
																childResourceApiVersion,
																parentNamespace)

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
				buildCompositions(resourceKind, resourceName, parentNamespace, level, compositionTree)
			}
		}
	} else {
		return
	}
}

func getAllNamespaces() []string {
	var url1 string
	url1 = fmt.Sprintf("https://%s:%s/%s/namespaces", serviceHost, servicePort, "api/v1")
	//fmt.Printf("URL:%s\n", url1)
	responseBody := queryAPIServer(url1)
	return parseNamespacesResponse(responseBody)
}

func (cp *ClusterCompositions) QueryResource(resourceKind, resourceName, namespace string) []byte {
	resourceApiVersion := kindVersionMap[resourceKind]
	resourceKindPlural := KindPluralMap[resourceKind]
	url1 := fmt.Sprintf("https://%s:%s/%s/namespaces/%s/%s/%s", serviceHost, servicePort, resourceApiVersion, namespace, resourceKindPlural, resourceName)
	fmt.Printf("Resource Query URL:%s\n", url1)
	contents := queryAPIServer(url1)
	return contents
}

func queryAPIServer(url1 string) []byte {
	if caToken != nil {
		caToken = getToken()
	}
	if caCertPool != nil {
		caCertPool = getCACert()
	}

	u, err := url.Parse(url1)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Close = true // to ensure that the connection is closed
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(caToken)))
	var client *http.Client

	if caCertPool != nil {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
		}
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("sending request failed: %s", err.Error())
		fmt.Println(err)
	}
	defer resp.Body.Close()

	resp_body, _ := ioutil.ReadAll(resp.Body)

	return resp_body
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

func parseNamespacesResponse(content []byte) []string {
	var result map[string]interface{}
	json.Unmarshal([]byte(content), &result)
	namespaces := make([]string, 0)
	items, ok := result["items"].([]interface{})

	if ok {
		for _, item := range items {
			itemConverted := item.(map[string]interface{})
			for key, value := range itemConverted {
				if key == "metadata" {
					metadataMap := value.(map[string]interface{})
					for mkey, mvalue := range metadataMap {
						if mkey == "name" {
							namespace := mvalue.(string)
							namespaces = append(namespaces, namespace)
						}
					}
				}
			}
		}
	}
	return namespaces
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
