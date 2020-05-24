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

func GetRelatives(relatives []string, connections [] Connection, level int, kind, instance, namespace string) ([]string, []Connection) {
	//relatives := make([]string, 0)
	err := readKindCompositionFile(kind)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return relatives, connections
	}
	exists := checkExistence(kind, instance, namespace)
	if exists {
		relStringList := relationshipMap[kind]
		//fmt.Printf("RelStringList:%s\n", relStringList)
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
			//relatives := make([]string, 0)
			relatives, connections = findRelatives(relatives, connections, level, kind, instance, namespace, relStringList)
		}
	}
	return relatives, connections
}

func checkExistence(kind, instance, namespace string) bool {
	dynamicClient, err := getDynamicClient()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
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
		fmt.Printf("Error: %s\n", err.Error())
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

func findRelatives(relatives []string, connections []Connection, level int, kind, instance, namespace string, relStringList []string) ([]string, []Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if relType == "label" {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames, relDetail := searchLabels(selectorLabelMap, targetKind, namespace)
				relatives, connections = buildGraph(relatives, connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == "specproperty" {
				relativesNames, relDetail := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind)
				relatives, connections = buildGraph(relatives, connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == "annotation" {
				relativesNames, relDetail := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind)
				relatives, connections = buildGraph(relatives, connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
		}
	}
	return relatives, connections
}

func buildGraph(relatives []string, connections []Connection, level int, relativesNames []string, targetKind, namespace, relType, relDetail string) ([]string, []Connection) {
	currentPeers, currentConnections := prepare(level, relativesNames, targetKind, namespace, relType, relDetail)
	unseenRelatives := filterRelatives(relatives, relativesNames)
	relatives, connections = appendCurrentLevelPeers(relatives, currentPeers, connections, currentConnections)
	nextLevelPeers, nextLevelConnections := searchNextLevel(relatives, connections, level, unseenRelatives, targetKind, namespace, relType, relDetail)
	relatives, connections = appendNextLevelPeers(relatives, nextLevelPeers, connections, nextLevelConnections)
	return relatives, connections
}

func filterRelatives(relatives, relativeNames []string) []string {
	relativesToSearch := make([]string,0)
	for _, relativeName := range relativeNames {
		found := false
		for _, currentRelative := range relatives {
			parts := strings.Split(currentRelative, " ")
			partsTwoName := strings.Split(parts[2], ":")[1]
			if partsTwoName == relativeName {
				found = true
			}
		}
		if !found {
			relativesToSearch = append(relativesToSearch, relativeName)
		}
	}
	return relativesToSearch
}

func appendCurrentLevelPeers(relatives, currentPeers []string, connections, currentConnections []Connection) ([]string, []Connection) {
	for _, rel := range currentPeers {
		relatives = append(relatives, rel)
	}
	for _, connection := range currentConnections {
		connections = append(connections, connection)
	}
	return relatives, connections
}

func appendNextLevelPeers(relatives, nextLevelPeers []string, connections, nextLevelConnections []Connection) ([]string, []Connection) {
	for _, nrel := range nextLevelPeers {
		present := false
		for _, rel := range relatives {
			if nrel == rel {
				present = true
			}
		}
		if !present {
			relatives = append(relatives, nrel)
		}
	}
	for _, nconnect := range nextLevelConnections {
		present := false
		for _, connect := range connections {
			if compareConnections(nconnect, connect) {
				present = true
			}
		}
		if !present {
			connections = append(connections, nconnect)
		}
	}
	return relatives, connections
}

func prepare(level int, relativeNames []string, targetKind, namespace, relType, relDetail string) ([]string, []Connection) {
	preparedRels := make([]string,0)
	preparedConnections := make([]Connection,0)
	for _, relativeName := range relativeNames {
		levelStr := strconv.Itoa(level)
		ownerDetail := getOwnerDetail(targetKind, relativeName, namespace)
		relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + relDetail + " " + ownerDetail
		preparedRels = append(preparedRels, relativeEntry)
		connection := Connection{
			Level: level,
			Kind: targetKind,
			Name: relativeName,
			Namespace: namespace,
			Owner: ownerDetail,
			RelationType: relType,
			RelationDetails: relDetail,
		}
		preparedConnections = append(preparedConnections, connection)
	}
	return preparedRels, preparedConnections
}

func searchNextLevel(relatives []string, connections []Connection, level int, relativeNames []string, targetKind, namespace, relType, relDetail string) ([]string, []Connection) {
	var subrelatives []string
	for _, relativeName := range relativeNames {
		subrelatives, connections = GetRelatives(relatives, connections, level, targetKind, relativeName, namespace)
	}
	return subrelatives, connections
}

func prepare_prev(relatives []string, level int, relativeNames []string, targetKind, namespace, relType, relDetail string) []string {
	fmt.Printf("((%v))\n", relatives)
	fmt.Printf("&&%v&&\n", relativeNames)
	relativesToSearch := make([]string,0)
	for _, relativeName := range relativeNames {
		found := false
		for _, currentRelative := range relatives {
			parts := strings.Split(currentRelative, " ")
			partsTwoName := strings.Split(parts[2], ":")[1]
			if partsTwoName == relativeName {
				found = true
			}
		}
		if !found {
			relativesToSearch = append(relativesToSearch, relativeName)
		}
	}
	for _, relativeName := range relativesToSearch {
		levelStr := strconv.Itoa(level)
		ownerDetail := getOwnerDetail(targetKind, relativeName, namespace)
		//fmt.Printf("Owner Detail:%s\n", ownerDetail)
		relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + relDetail + " " + ownerDetail
		//fmt.Printf("%s\n", relativeEntry)
		present := false
		for _, r := range relatives {
			if r == relativeEntry {
				present = true
			}
		}
		if !present {
			relatives = append(relatives, relativeEntry)
		}
	}
	return relatives
}

func prepareAndSearchNextLevel(relatives []string, connections []Connection, level int, relativeNames []string, targetKind, namespace, relType, relDetail string) []string {
	//fmt.Printf("((%v))\n", relatives)
	//fmt.Printf("&&%v&&\n", relativeNames)
	relativesToSearch := make([]string,0)
	for _, relativeName := range relativeNames {
		found := false
		for _, currentRelative := range relatives {
			parts := strings.Split(currentRelative, " ")
			partsTwoName := strings.Split(parts[2], ":")[1]
			if partsTwoName == relativeName {
				found = true
			}
		}
		if !found {
			relativesToSearch = append(relativesToSearch, relativeName)
		}
	}
	for _, relativeName := range relativesToSearch {
		levelStr := strconv.Itoa(level)
		ownerDetail := getOwnerDetail(targetKind, relativeName, namespace)
		//fmt.Printf("Owner Detail:%s\n", ownerDetail)
		relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + relDetail + " " + ownerDetail
		//fmt.Printf("%s\n", relativeEntry)
		present := false
		for _, r := range relatives {
			if r == relativeEntry {
				present = true
			}
		}
		if !present {
			relatives = append(relatives, relativeEntry)
		}
	}
	var subrelatives []string
	for _, relativeName := range relativesToSearch {
		subrelatives, connections = GetRelatives(relatives, connections, level, targetKind, relativeName, namespace)
	}
	for _, subrelative := range subrelatives {
		relatives = append(relatives, subrelative)
	}
	return relatives
}

func getOwnerDetail(kind, instance, namespace string) string {
	oKind := ""
	oInstance := ""
	ownerKind, ownerInstance := findOwner(kind, instance, namespace)
	if ownerKind != kind && ownerInstance != instance {
		oKind = ownerKind
		oInstance = ownerInstance
	}
	ownerDetail := "Owner:" + oKind + "/" + oInstance
	return ownerDetail
}

func findOwner(kind, instance, namespace string) (string, string) {
	ownerKind := ""
	ownerInstance := ""
	ownerResKindPlural, _, ownerResApiVersion, ownerResGroup := getKindAPIDetails(kind)
	ownerRes := schema.GroupVersionResource{Group: ownerResGroup,
									 		Version: ownerResApiVersion,
									   		Resource: ownerResKindPlural}
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return kind, instance
	}
	instanceObj, err := dynamicClient.Resource(ownerRes).Namespace(namespace).Get(context.TODO(),
																			 	  instance,
																	   		 	  metav1.GetOptions{})
	if err != nil {
		return kind, instance
	}
	ownerReference := instanceObj.GetOwnerReferences()
	if len(ownerReference) == 0 {
		ownerKind = kind
		ownerInstance = instance
	} else {
		owner := ownerReference[0]
		oKind := owner.Kind
		oName := owner.Name
		//fmt.Printf("oKind:%s, oName:%s\n", oKind, oName)
		ownerKind, ownerInstance = findOwner(oKind, oName, namespace)
		if ownerKind == kind {
			ownerKind = ""
		}
		if ownerInstance == instance {
			ownerInstance = ""
		}
	}
	return ownerKind, ownerInstance
}

func searchAnnotations(kind, instance, namespace, annotationKey, annotationValue, targetKind string) ([]string, string) {
	relativesNames := make([]string, 0)
	relDetail := ""
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames, relDetail
	}
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	_, err = dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames, relDetail
	}
	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return relativesNames, relDetail
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
	relDetail = annotationKey + "::" + annotationValue
	return relativesNames, relDetail
}

func searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind string) ([]string, string) {
	relativesNames := make([]string, 0)
	envNameValue := ""
	if lhs == "env" {
		relativesNames, envNameValue = searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind)
	} else {
		relativesNames, envNameValue = searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind)		
	}
	return relativesNames, envNameValue
}

func searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind string) ([]string, string) {
	relativesNames := make([]string, 0)
	propertyNameValue := ""
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames, propertyNameValue
	}
	instanceObj, err := dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames, propertyNameValue
	}
	lhsContent := instanceObj.UnstructuredContent()

	fieldValue, found, err := unstructured.NestedString(lhsContent, "spec", lhs)
	//fmt.Printf("FieldValue:%s, found:%v, Error:%v", fieldValue, found, err)
	if err != nil || !found {
		return relativesNames, propertyNameValue
	}
	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return relativesNames, propertyNameValue
	}
	for _, unstructuredObj := range rhsInstList.Items {
		if rhs == "name" {
			rhsInstanceName := unstructuredObj.GetName()
			if fieldValue == rhsInstanceName {
				//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
				present := false
				for _, relName := range relativesNames {
					if relName == rhsInstanceName {
						present = true
						break
					}
				}
				if !present {
					relativesNames = append(relativesNames, rhsInstanceName)
				}
				//relativesNames = append(relativesNames, rhsInstanceName)
				propertyNameValue = "Name:" + lhs + " " + "Value:" + fieldValue
			}
		}
	}
	return relativesNames, propertyNameValue
}

func searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind string) ([]string, string) {
	relativesNames := make([]string, 0)
	envNameValue := ""
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames, envNameValue
	}
	instanceObj, err := dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames, envNameValue
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
		return relativesNames, envNameValue
	}
	containerList, ok, _ := unstructured.NestedSlice(lhsContent, "spec", "containers")
	if ok {
		for _, cont := range containerList {
			container := cont.(map[string]interface{})
			envVarList, ok, _ := unstructured.NestedSlice(container,"env")
			if ok {
				for _, envVar := range envVarList {
					envMap := envVar.(map[string]interface{})
					envName := ""
					envValue := ""
					if val, ok := envMap["name"]; ok {
						envName = val.(string)
					}
					//envName := envMap["name"].(string)
					if val, ok := envMap["value"]; ok {
						envValue = val.(string)
					}
					//envValue := envMap["value"].(string)
					//fmt.Printf("Name:%s, Value:%s", envName, envValue)
					for _, unstructuredObj := range rhsInstList.Items {
						//fmt.Printf(" Service name:%s\n", unstructuredObj.GetName())
						if rhs == "name" {
							rhsInstanceName := unstructuredObj.GetName()
							if envValue == rhsInstanceName {
								//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
								present := false
								for _, relName := range relativesNames {
									if relName == rhsInstanceName {
										present = true
										break
									}
								}
								if !present {
									relativesNames = append(relativesNames, rhsInstanceName)
								}
								envNameValue = "Name:" + envName + " " + "Value:" + envValue
							}
						}
					}
				}
			}
		}
	}
	return relativesNames, envNameValue
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
				if strings.Contains(value, instanceName) {
					parts := strings.Split(value, ":")
					for _, part := range parts {
						fmt.Printf("--- FOUND --- value:%s part:%s\n", value, part)
						return true
					}
				}
		}
	}
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
		match := subsetMatchMaps(labelMap, selectorMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames
}

func searchLabels(labelMap map[string]string, targetKind, namespace string) ([]string, string) {
	instanceNames := make([]string, 0)
	relDetail := ""
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return instanceNames, relDetail
	}
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(targetKind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}
	list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return instanceNames, relDetail
	}
	for _, unstructuredObj := range list.Items {
		unstructuredObjLabelMap := unstructuredObj.GetLabels()
		match := subsetMatchMaps(labelMap, unstructuredObjLabelMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
		}
	}
	for key, value := range labelMap {
		relDetail = relDetail + key + ":" + value + " "
	}
	return instanceNames, relDetail
}

func subsetMatchMaps(map1, map2 map[string]string) bool {
	if len(map1) == 0 {
		return false
	}
	for key, element := range map1 {
		value, found := map2[key]
		if !found {
			return false
		}
		if value != element {
			return false
		}
	}
	return true
}