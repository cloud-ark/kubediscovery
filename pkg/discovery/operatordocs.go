package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coreos/etcd/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getCRDNames(crdListString string) []string {
	var operatorMapList []map[string]map[string]interface{}
	var operatorDataMap map[string]interface{}

	if err := json.Unmarshal([]byte(crdListString), &operatorMapList); err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	var crdNameList []string = make([]string, 0)
	for _, operator := range operatorMapList {
		operatorDataMap = operator["Operator"]

		customResources := operatorDataMap["CustomResources"]

		for _, cr := range customResources.([]interface{}) {
			crdNameList = append(crdNameList, cr.(string))
		}
	}
	return crdNameList
}

func getCRDDetails(crdDetailsString string) (string, string, string, []string, string, string, string) {

	var crdDetailsMap = make(map[string]interface{})
	kind := ""
	plural := ""
	endpoint := ""
	composition := make([]string, 0)

	if err := json.Unmarshal([]byte(crdDetailsString), &crdDetailsMap); err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	kind = crdDetailsMap["kind"].(string)
	endpoint = crdDetailsMap["endpoint"].(string)
	plural = crdDetailsMap["plural"].(string)

	compositionString := crdDetailsMap["composition"].(string)
	composition1 := strings.Split(compositionString, ",")
	for _, elem := range composition1 {
		elem = strings.TrimSpace(elem)
		composition = append(composition, elem)
	}

	implementationChoicesCMapName := crdDetailsMap["constants"].(string)
	usageCMapName := crdDetailsMap["usage"].(string)
	openapiSpecCMapName := crdDetailsMap["openapispec"].(string)

	return kind, plural, endpoint, composition, implementationChoicesCMapName, usageCMapName, openapiSpecCMapName
}

func GetUsageDetails(customResourceKind string) (string, error) {
	var usageDetailsData string
	var kind, usageDetailsCMapName string
	crdNameList, err := queryETCDNodes("/crds")
	fmt.Printf("CRD Name List:%v", crdNameList)
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}
	for _, crdName := range crdNameList {
		crdDetailsString, err := queryETCD("/" + crdName)
		if err != nil {
			fmt.Printf("Error:%s\n", err.Error())
			return "", err
		}
		kind, _, _, _, _, usageDetailsCMapName, _ = getCRDDetails(crdDetailsString)

		if kind == customResourceKind {
			usageDetailsData, err = readConfigMap(usageDetailsCMapName)
			if err != nil {
				fmt.Printf("Error:%s\n", err.Error())
				usageDetailsData = "Could not find usage details data."
			}
			fmt.Printf("Usage Details:%v", usageDetailsData)
			return usageDetailsData, err
		}
	}
	return "", err
}

func GetImplementationDetails(customResourceKind string) (string, error) {
	var implementationDetailsData string
	var kind, implementationDetailsCMapName string
	crdNameList, err := queryETCDNodes("/crds")
	fmt.Printf("CRD Name List:%v", crdNameList)
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}
	for _, crdName := range crdNameList {
		crdDetailsString, err := queryETCD("/" + crdName)
		if err != nil {
			fmt.Printf("Error:%s\n", err.Error())
			return "", err
		}
		kind, _, _, _, implementationDetailsCMapName, _, _ = getCRDDetails(crdDetailsString)
		fmt.Printf(":::: Implementation Details CMap:%s ::::", implementationDetailsCMapName)

		if kind == customResourceKind {
			implementationDetailsData, err = readConfigMap(implementationDetailsCMapName)
			if err != nil {
				fmt.Printf("Error:%s\n", err.Error())
				implementationDetailsData = "Could not find implementation details data."
			}
			fmt.Printf("Implementation Details:%v", implementationDetailsData)
			return implementationDetailsData, err
		}
	}
	return "", err
}

func GetOpenAPISpec(customResourceKind string) (string, error) {

	var openapiData string
	var kind, openapispecCMapName string
	crdNameList, err := queryETCDNodes("/crds")
	fmt.Printf("CRD Name List:%v", crdNameList)
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}
	for _, crdName := range crdNameList {
		crdDetailsString, err := queryETCD("/" + crdName)
		if err != nil {
			fmt.Printf("Error:%s\n", err.Error())
			return "", err
		}
		kind, _, _, _, _, _, openapispecCMapName = getCRDDetails(crdDetailsString)

		if kind == customResourceKind {
			openapiData, err = readConfigMap(openapispecCMapName)
			if err != nil {
				fmt.Printf("Error:%s\n", err.Error())
				openapiData = "Could not find implementation details data."
			}
			fmt.Printf("Implementation Details:%v", openapiData)
			return openapiData, err
		}
	}
	return "", err
}

func GetOpenAPISpec_prev(customResourceKind string) string {

	// 1. Get ConfigMap Name by querying etcd at
	resourceKey := "/" + customResourceKind + "-OpenAPISpecConfigMap"
	configMapNameString, err := queryETCD(resourceKey)

	var configMapName string
	if err := json.Unmarshal([]byte(configMapNameString), &configMapName); err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	// 2. Query ConfigMap
	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	configMap, err := kubeClient.CoreV1().ConfigMaps("default").Get(configMapName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	configMapData := configMap.Data
	openAPISpec := configMapData["openapispec"]

	return openAPISpec
}

func readConfigMap(implementationDetailsString string) (string, error) {

	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}

	fields := strings.Split(implementationDetailsString, ".")

	namespace := "default"
	var configMapName, dataFieldName string
	if len(fields) >= 3 {
		namespace = fields[0]
		configMapName = fields[1]
		dataFieldName = fields[2]
	} else {
		configMapName = fields[0]
		dataFieldName = fields[1]
	}

	fmt.Printf("Namespace:%s, configMapName:%s, dataFieldName:%s", namespace, configMapName, dataFieldName)

	configMap, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Error:%s\n", err.Error())
		return "", err
	}

	configMapData := configMap.Data
	data := configMapData[dataFieldName]

	fmt.Printf("Data:%s", data)

	return data, nil
}

func queryETCDNodes(resourceKey string) ([]string, error) {
	cfg := client.Config{
		Endpoints: []string{etcdServiceURL},
		Transport: client.DefaultTransport,
	}
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)

	crdList := make([]string, 0)

	resp, err1 := kapi.Get(context.Background(), resourceKey, nil)
	if err1 != nil {
		return crdList, err1
	} else {
		sort.Sort(resp.Node.Nodes)
		for _, n := range resp.Node.Nodes {
			crdList = append(crdList, n.Key)
		}
		return crdList, nil
	}
}

func queryETCD(resourceKey string) (string, error) {
	cfg := client.Config{
		Endpoints: []string{etcdServiceURL},
		Transport: client.DefaultTransport,
	}
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)

	resp, err1 := kapi.Get(context.Background(), resourceKey, nil)
	if err1 != nil {
		return "", err1
	} else {
		return resp.Node.Value, nil
	}
}
