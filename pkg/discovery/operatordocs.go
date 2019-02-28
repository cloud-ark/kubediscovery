package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coreos/etcd/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

func getCRDDetails(crdDetailsString string) (string, string, string, []string) {

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

	return kind, plural, endpoint, composition
}

func GetOpenAPISpec(customResourceKind string) string {

	// 1. Get ConfigMap Name by querying etcd at
	resourceKey := "/" + customResourceKind + "-OpenAPISpecConfigMap"
	configMapNameString, err := queryETCD(resourceKey)

	var configMapName string
	if err := json.Unmarshal([]byte(configMapNameString), &configMapName); err != nil {
		fmt.Printf("Error:%s\n", err.Error())
	}

	// 2. Query ConfigMap
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
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
