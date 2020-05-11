package discovery

import (
	"strings"
	"fmt"
	"strconv"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func GetRelatives(level int, kind, instance, namespace string) []string{
	relatives := make([]string, 0)
	err := readKindCompositionFile(kind)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return relatives
	}
	exists := checkExistence(kind, instance, namespace)
	if exists {
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
			level = level + 1
			//fmt.Printf("%d\n", level)
				relatives = findRelatives(level, kind, instance, namespace, relStringList)
			}
	}
	return relatives
}

func checkExistence(kind, instance, namespace string) bool {
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return false
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(kind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	_, err = dynamicClient.Resource(res).Namespace(namespace).Get(context.TODO(),
																			 instance,
																	   		 metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

func findInverseRelatives(kind, instance, namespace, targetKind string, relStringList []string) []string {
	relatives := make([]string, 0)
	for _, relString := range relStringList {
		relType, _, _, _ := parseRelationship(relString)
		//fmt.Printf("TargetKinds:%s\n", targetKindList)
		//for _, targetKind := range targetKindList {
			//fmt.Printf("%s, %s, %s\n", relType, relValue, targetKind)
			if relType == "label" {
				labelMap := getLabels(kind, instance, namespace)
				//selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames := searchSelectors(labelMap, targetKind, namespace)
				for _, relativeName := range relativesNames {
					relativeEntry := "kind:" + targetKind + " name:" + relativeName +  " relationship-type:label" 
					relatives = append(relatives, relativeEntry)
				}
			}
			if relType == "specProperty" {

			}
		//}
	}
	return relatives
}

func findRelatedKinds(kind string) []string{
	relatedKinds := make([]string, 0)
	for key, relStringList := range relationshipMap {
		for _, relString := range relStringList {
			_, _, _, targetKindList := parseRelationship(relString)
			for _, targetKind := range targetKindList {
				if targetKind == kind {
					relatedKinds = append(relatedKinds, key)
				}
			}
		}
	}
	return relatedKinds
}

func findRelatives(level int, kind, instance, namespace string, relStringList []string) []string {
	relatives := make([]string, 0)
	//fmt.Printf("--- %v\n", relStringList)
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		//fmt.Printf("TargetKinds:%s\n", targetKindList)
		for _, targetKind := range targetKindList {
			//fmt.Printf("%s, %s, %s\n", relType, relValue, targetKind)
			if relType == "label" {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				//fmt.Printf("SelectorLabelMap:%s\n", selectorLabelMap)
				relativesNames := searchLabels(selectorLabelMap, targetKind, namespace)
				//fmt.Printf("Label RelativesNames:%s\n", relativesNames)
				relativesLabel := prepareAndSearchNextLevel(level, relativesNames, targetKind, namespace, relType)
				for _, rel := range relativesLabel {
					relatives = append(relatives, rel)
				}
			}
			if relType == "specproperty" {
				relativesNames := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind)
				//fmt.Printf("SpecProperty RelativesNames:%s\n", relativesNames)
				relativesSpec := prepareAndSearchNextLevel(level, relativesNames, targetKind, namespace, relType)
				for _, rel := range relativesSpec {
					relatives = append(relatives, rel)
				}
			}
			if relType == "annotation" {
				//fmt.Printf("Inside checking annotations..\n")
				relativesNames := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind)
				relativesSpec := prepareAndSearchNextLevel(level, relativesNames, targetKind, namespace, relType)
				for _, rel := range relativesSpec {
					relatives = append(relatives, rel)
				}
			}
		}
	}
	return relatives
}

func prepareAndSearchNextLevel(level int, relativeNames []string, targetKind, namespace, relType string) []string {
	relatives := make([]string,0)
	for _, relativeName := range relativeNames {
		levelStr := strconv.Itoa(level)
		relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " relationship-type:" + relType
		//fmt.Printf("%s\n", relativeEntry)
		relatives = append(relatives, relativeEntry)
	}
	var subrelatives []string
	for _, relativeName := range relativeNames {
		subrelatives = GetRelatives(level, targetKind, relativeName, namespace)
	}
	for _, subrelative := range subrelatives {
		relatives = append(relatives, subrelative)
	}
	return relatives
}

func searchAnnotations(kind, instance, namespace, annotationKey, annotationValue, targetKind string) []string {
	relativesNames := make([]string, 0)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames
	}
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	_, err = dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames
	}
	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return relativesNames
	}
	for _, unstructuredObj := range rhsInstList.Items {
		//rhsContent := unstructuredObj.UnstructuredContent()
		//annotationMap, ok, _ := unstructured.NestedMap(rhsContent, "annotations")
		annotations := unstructuredObj.GetAnnotations()
		//fmt.Printf("AnnotationMap:%v\n", annotations)
		for key, value := range annotations {
			if key == annotationKey && value == instance {
				rhsInstanceName := unstructuredObj.GetName()
					//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
				relativesNames = append(relativesNames, rhsInstanceName)
			}
		}
	}
	return relativesNames
}

func searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind string) []string {
	relativesNames := make([]string, 0)
	if lhs == "env" {
		relativesNames = searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind)
	}
	return relativesNames
}

func searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind string) []string {
	relativesNames := make([]string, 0)
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames
	}
	instanceObj, err := dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames
	}
	lhsContent := instanceObj.UnstructuredContent()
	//jsonContent, _ := instanceObj.MarshalJSON()
	//fmt.Printf("JSON Content:%s\n", string(jsonContent))
	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return relativesNames
	}
	containerList, ok, _ := unstructured.NestedSlice(lhsContent, "spec", "containers")
	if ok {
		for _, cont := range containerList {
			container := cont.(map[string]interface{})
			envVarList, ok, _ := unstructured.NestedSlice(container,"env")
			if ok {
				for _, envVar := range envVarList {
					envMap := envVar.(map[string]interface{})
					//envName := envMap["name"]
					envValue := envMap["value"]
					//fmt.Printf("Name:%s, Value:%s\n", envName, envValue)
					for _, unstructuredObj := range rhsInstList.Items {
						if rhs == "name" {
							rhsInstanceName := unstructuredObj.GetName()
							if envValue == rhsInstanceName {
								//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
								relativesNames = append(relativesNames, rhsInstanceName)
							}
						}
					}
				}
			}
		}
	}
	return relativesNames
}

func checkContent(lhsContent interface{}, instanceName string) bool {
	//var content interface{}
	_, ok1 := lhsContent.(map[string]interface{})
	if ok1 {
		content := lhsContent.(map[string]interface{})
		for _, value := range content {
			//fmt.Printf("Key: Value:%s\n", key, value)
			stringVal, ok := value.(string)
			if ok {
				//if instanceName == "wordpress-mysql" {
				//	fmt.Printf("Key:%s, Value:%s, InstanceName:%s\n", key, value, instanceName)				
				//}
				//if strings.Contains(stringVal, instanceName)  {
				if stringVal == instanceName {
					fmt.Printf("*** FOUND *** value:%s\n", stringVal)
					return true
				}
			} else {
				return checkContent(value, instanceName)
			}
		}
	}
	_, ok2 := lhsContent.([]string)
	if ok2 {
		content := lhsContent.([]string)
		for _, value := range content {
			//fmt.Printf("Key: Value:%s\n", key, value)
			//_, ok := value.(string)
			//if ok {
				//if instanceName == "wordpress-mysql" {
				//	fmt.Printf("Key:%s, Value:%s, InstanceName:%s\n", key, value, instanceName)				
				//}
				if strings.Contains(value, instanceName) {
					parts := strings.Split(value, ":")
					for _, part := range parts {
						fmt.Printf("--- FOUND --- value:%s part:%s\n", value, part)
						return true
					}
				}
			//} else {
			//	return checkContent(value, instanceName)
			//}
		}
	}
	/*for key, value := range lhsContent {
		fmt.Printf("Value:%s\n", value)
		_, ok := value.(string)
		if ok {
			if instanceName == "wordpress-mysql" {
				fmt.Printf("Key:%s, Value:%s, InstanceName:%s\n", key, value, instanceName)				
			}
			if value == instanceName {
				return true
			}
		}
		_, ok = value.(map[string]interface{})
		if ok {
			return checkContent(value.(map[string]interface{}), instanceName)
		}
	}*/
	return false
}

func parseRelationship(relString string) (string, string, string, []string) {
	// Update this method to parse out the kinds based on relationship types: label vs. specproperty
	targetKindList := make([]string, 0)
	parts := strings.Split(relString, ",")
	relType := strings.TrimSpace(parts[0])
	var lhs, rhs string
	if relType == "label" {
		targetKind := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhs = strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		//targetKindList = strings.Split(targetKinds, ";")
		targetKindList = append(targetKindList, targetKind)
	}
	if relType == "specproperty" {
		targetKindString := strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		targetKindStringParts := strings.Split(targetKindString, ".")
		targetKind := targetKindStringParts[0]
		targetKindList = append(targetKindList, targetKind)
		rhs = targetKindStringParts[len(targetKindStringParts)-1]

		lhsString := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhsStringParts := strings.Split(lhsString, ".")
		lhs = lhsStringParts[len(lhsStringParts)-1]
	}
	if relType == "annotation" {
		targetKind := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhs = strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		rhs = strings.Split(strings.TrimSpace(parts[3]), ":")[1]
		targetKindList = append(targetKindList, targetKind)
	}
	//fmt.Printf("RelType:%s, lhs:%s, rhs:%s TargetKindList:%s\n", relType, lhs, rhs, targetKindList)
	return relType, lhs, rhs, targetKindList
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
		match := subsetMatchMaps(labelMap, selectorMap)
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
		//fmt.Printf("Obj Label Map:%s\n", unstructuredObjLabelMap)
		match := subsetMatchMaps(labelMap, unstructuredObjLabelMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames
}

func subsetMatchMaps(map1, map2 map[string]string) bool {
	for key, element := range map1 {
		value, found := map2[key]
		if !found {
			return false
		}
		if value != element {
			return false
		}
	}
/*
	for key, element := range map2 {
		value, found := map1[key]
		if !found {
			return false
		}
		if value != element {
			return false
		}
	}
*/
	return true
}