package discovery

import (
	"strings"
	"fmt"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func GetRelatives(kind, instance, namespace string) []string{
	relatives := make([]string, 0) //siblings entry: relationship: label, kind: Pod, name: <Pod-name>
	relStringList := relationshipMap[kind]
	if len(relStringList) == 0 {
		kindList := findRelatedKinds(kind)
		for _, targetKind := range kindList {
			relationshipStringList := relationshipMap[targetKind]
			inverseRelatives := findInverseRelatives(kind,
													 instance,
													 namespace, 
													 targetKind,
													 relationshipStringList)
			for _, inverseRelative := range inverseRelatives {
				relatives = append(relatives, inverseRelative)
			}
		}
	} else {
		relatives = findRelatives(kind, instance, namespace, relStringList)
	}
	return relatives
}

func findInverseRelatives(kind, instance, namespace, targetKind string, relStringList []string) []string {
	relatives := make([]string, 0)
	for _, relString := range relStringList {
		relType, _, _ := parseRelationship(relString)
		//fmt.Printf("TargetKinds:%s\n", targetKindList)
		//for _, targetKind := range targetKindList {
			//fmt.Printf("%s, %s, %s\n", relType, relValue, targetKind)
			if relType == "label" {
				labelMap := getLabels(kind, instance, namespace)
				//selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames := searchSelectors(labelMap, targetKind, namespace)
				for _, relativeName := range relativesNames {
					relativeEntry := "kind:" + targetKind + ", name:" + relativeName +  " relationship-type:label" 
					relatives = append(relatives, relativeEntry)
				}
			}
		//}
	}
	return relatives
}

func findRelatedKinds(kind string) []string{
	relatedKinds := make([]string, 0)
	for key, relStringList := range relationshipMap {
		for _, relString := range relStringList {
			_, _, targetKindList := parseRelationship(relString)
			for _, targetKind := range targetKindList {
				if targetKind == kind {
					relatedKinds = append(relatedKinds, key)
				}
			}
		}
	}
	return relatedKinds
}

func findRelatives(kind, instance, namespace string, relStringList []string) []string {
	relatives := make([]string, 0)
	for _, relString := range relStringList {
		relType, _, targetKindList := parseRelationship(relString)
		//fmt.Printf("TargetKinds:%s\n", targetKindList)
		for _, targetKind := range targetKindList {
			//fmt.Printf("%s, %s, %s\n", relType, relValue, targetKind)
			if relType == "label" {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames := searchLabels(selectorLabelMap, targetKind, namespace)
				for _, relativeName := range relativesNames {
					relativeEntry := "kind:" + targetKind + ", name:" + relativeName +  " relationship-type:label" 
					relatives = append(relatives, relativeEntry)
				}
			}
		}
	}
	return relatives
}

func parseRelationship(relString string) (string, string, []string) {
	targetKindList := make([]string, 0)
	parts := strings.Split(relString, ",")
	relType := strings.TrimSpace(parts[0])
	relValue := strings.Split(strings.TrimSpace(parts[2]), ":")[1]
	targetKinds := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
	targetKindList = strings.Split(targetKinds, ";")
	return relType, relValue, targetKindList
}

func getLabels(kind, instance, namespace string) map[string]string {
	labelMap := make(map[string]string)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		fmt.Printf(err.Error())
		return labelMap
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(kind)
	//fmt.Printf("%s, %s, %s\n", resourceGroup, resourceApiVersion, resourceKindPlural)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	instanceObj, err := dynamicClient.Resource(res).Namespace(namespace).Get(context.TODO(),
																			 instance,
																	   		 metav1.GetOptions{})
	if err != nil {
		fmt.Printf(err.Error())
		return labelMap
	}
	labelMap = instanceObj.GetLabels()
	return labelMap
}

func getSelectorLabels(kind, instance, namespace string) map[string]string {
	selectorMap := make(map[string]string)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		fmt.Printf(err.Error())
		return selectorMap
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(kind)
	//fmt.Printf("%s, %s, %s\n", resourceGroup, resourceApiVersion, resourceKindPlural)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	instanceObj, err := dynamicClient.Resource(res).Namespace(namespace).Get(context.TODO(),
																			 instance,
																	   		 metav1.GetOptions{})
	if err != nil {
		fmt.Printf(err.Error())
		return selectorMap
	}
	content := instanceObj.UnstructuredContent()
	selectorMap, _, _ = unstructured.NestedStringMap(content, "spec", "selector")
	//fmt.Printf("SelectorMap:%s\n", selectorMap)
	return selectorMap
}

func searchSelectors(labelMap map[string]string, targetKind, namespace string) []string {
	instanceNames := make([]string, 0)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return instanceNames
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(targetKind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return instanceNames
	}
	for _, unstructuredObj := range list.Items {
		content := unstructuredObj.UnstructuredContent()
		selectorMap, _, _ := unstructured.NestedStringMap(content, "spec", "selector")
		//fmt.Printf("%s\n", unstructuredObjLabelMap)
		match := compareMaps(labelMap, selectorMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames
}

func searchLabels(labelMap map[string]string, targetKind, namespace string) []string {
	instanceNames := make([]string, 0)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return instanceNames
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(targetKind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return instanceNames
	}
	for _, unstructuredObj := range list.Items {
		unstructuredObjLabelMap := unstructuredObj.GetLabels()
		//fmt.Printf("%s\n", unstructuredObjLabelMap)
		match := compareMaps(labelMap, unstructuredObjLabelMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames
}

func compareMaps(map1, map2 map[string]string) bool {
	for key, element := range map1 {
		value, found := map2[key]
		if !found {
			return false
		}
		if value != element {
			return false
		}
	}
	for key, element := range map2 {
		value, found := map1[key]
		if !found {
			return false
		}
		if value != element {
			return false
		}
	}
	return true
}