package discovery

import (
	"strings"
	"fmt"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/dynamic"
)

func GetRelatives(connections [] Connection, level int, kind, instance, namespace string) ([]Connection) {
	err := readKindCompositionFile(kind)
	//fmt.Printf("RelationshipMap:%v\n", relationshipMap)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return connections
	}
	exists := checkExistence(kind, instance, namespace)
	if exists {
		relatedKindList := findRelatedKinds(kind)
		//fmt.Printf("Level: %d, Input Kind: %s RelatedKindList:%v\n",level, kind,relatedKindList)
		connections = findRelatives(connections, level, kind, instance, namespace, relatedKindList)
	}
	return connections
}

func checkExistence(kind, instance, namespace string) bool {
	//fmt.Printf("Kind:%s, instance:%s, namespace:%s\n", kind, instance, namespace)
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
		_, err1 := dynamicClient.Resource(res).Get(context.TODO(),instance,metav1.GetOptions{})
		if err1 != nil {
			panic(err1)
			return false
		}
	}
	return true
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

func findChildKinds(kind string) []string {
	childKinds := make([]string, 0)
	for _, relStringList := range relationshipMap {
		for _, relString := range relStringList {
			relType, _, _, targetKindList := parseRelationship(relString)
			if relType == "owner" {
				for _, tk := range targetKindList {
					childKinds = append(childKinds, tk)
				}
			}
		}
	}
	return childKinds
}

func findRelatives(connections []Connection, level int, kind, instance, namespace string, relatedKindList []string) ([]Connection) {
	// Put self in the list of relations so that we don't traverse back to self.
	relativesNames := make([]string,0)
	relativesNames = append(relativesNames, instance)
	unseenRelatives := filterRelatives(connections, relativesNames)
	currentConnections, _ := prepare(level, unseenRelatives, kind, namespace, "", "")
	connections = appendCurrentLevelPeers(connections, currentConnections, level)
	level = level + 1
	relStringList := relationshipMap[kind]
	connections = findDownstreamRelatives(connections, level, kind, instance, namespace, relStringList)
	for _, relatedKind := range relatedKindList {
		relStringList = relationshipMap[relatedKind]
		connections = findUpstreamRelatives(connections, level, relatedKind, instance, namespace, relStringList)
	}
	//connections = searchOwnerGraph(connections, owners, level)
	connections = findParentConnections(connections, level, kind, instance, namespace)
	connections = findChildrenConnections(connections, level, kind, instance, namespace)
	return connections
}

func findDownstreamRelatives(connections []Connection, level int, kind, instance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if relType == "label" {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames, relDetail := searchLabels(selectorLabelMap, targetKind, namespace)
				//fmt.Printf("FDSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == "specproperty" {
				targetInstance := "*"
				relativesNames, relDetail := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == "annotation" {
				targetInstance := "*"
				relativesNames, relDetail := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
				connections = buildGraph(connections, level, relativesNames, targetKind, namespace, relType, relDetail)
			}
		}
	}
	return connections
}

func findUpstreamRelatives(connections []Connection, level int, kind, targetInstance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if relType == "label" {
				labelMap := getLabels(targetKind, targetInstance, namespace)
				relativesNames, relDetail := searchSelectors(labelMap, kind, namespace)
				//fmt.Printf("FUSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, relativesNames, kind, namespace, relType, relDetail)
			}
			if relType == "specproperty" {
				instance := "*"
				relativesNames, relDetail := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FUSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, relativesNames, kind, namespace, relType, relDetail)
			}
			if relType == "annotation" {
				instance := "*"
				relativesNames, relDetail := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
				connections = buildGraph(connections, level, relativesNames, kind, namespace, relType, relDetail)
			}
		}
	}
	return connections
}

func buildGraph(connections []Connection, level int, relativesNames []string, targetKind, namespace, relType, relDetail string) ([]Connection) {
	currentConnections, _ := prepare(level, relativesNames, targetKind, namespace, relType, relDetail)
	unseenRelatives := filterRelatives(connections, relativesNames)
	connections = appendCurrentLevelPeers(connections, currentConnections, level)	
	nextLevelConnections := searchNextLevel(connections, level, unseenRelatives, targetKind, namespace)
	connections = appendNextLevelPeers(connections, nextLevelConnections)
	//connections = searchOwnerGraph(connections, owners, level)
	//connections = findParentConnections(connections, level, kind, instance, namespace)
	//connections = findChildrenConnections(connections, level, kind, instance, namespace)
	return connections
}

func filterRelatives(connections []Connection, relativeNames []string) []string {
	relativesToSearch := make([]string,0)
	for _, relativeName := range relativeNames {
		found := false
		for _, currentRelative := range connections {
			if currentRelative.Name == relativeName {
				found = true
			}
		}
		if !found {
			relativesToSearch = append(relativesToSearch, relativeName)
		}
	}
	return relativesToSearch
}

func filterConnections(allConnections []Connection, currentConnections []Connection) []Connection {
	connectionsToSearch := make([]Connection,0)
	for _, currentConn := range currentConnections {
		found := false
		for _, conn := range allConnections {
			if currentConn.Name == conn.Name && 
			   currentConn.Kind == conn.Kind && 
			   currentConn.Namespace == conn.Namespace {
				found = true
				if currentConn.Level < conn.Level {
					conn.Level = currentConn.Level
					//allConnections[j] = conn
				}
			}
		}
		if !found {
			connectionsToSearch = append(connectionsToSearch, currentConn)
		}
	}
	return connectionsToSearch
}

func appendCurrentLevelPeers(connections, currentConnections []Connection, level int) ([]Connection) {
	connections = appendConnections(connections, currentConnections, level)
	return connections
}

func appendNextLevelPeers(connections, nextLevelConnections []Connection) ([]Connection) {
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
	return connections
}

func prepare(level int, relativeNames []string, targetKind, namespace, relType, relDetail string) ([]Connection, []Connection) {
	preparedConnections := make([]Connection,0)
	owners := make([]Connection,0)
	for _, relativeName := range relativeNames {
		ownerKind, ownerInstance := getOwnerDetail(targetKind, relativeName, namespace)
		ownerDetail := "Owner:" + ownerKind + "/" + ownerInstance
		connection := Connection{
			Level: level,
			Kind: targetKind,
			Name: relativeName,
			Namespace: namespace,
			Owner: ownerDetail,
			OwnerKind: ownerKind,
			OwnerName: ownerInstance,
			RelationType: relType,
			RelationDetails: relDetail,
		}
		preparedConnections = append(preparedConnections, connection)

		//Add Owner to the preparedConnections as well
		if ownerKind != "" && ownerInstance != "" {
			ownerConnection := Connection{
				Level: level+1,
				Kind: ownerKind,
				Name: ownerInstance,
				Namespace: namespace,
				RelationType: "owner",
			}
			owners = append(owners, ownerConnection)
		}
	}
	return preparedConnections, owners
}

func findParentConnections(connections []Connection, level int, kind, instance, namespace string) []Connection {
	ownerKind, ownerInstance := getOwnerDetail(kind, instance, namespace)
	ownerConn := Connection{
		Name: ownerInstance,
		Kind: ownerKind,
		Namespace: namespace,
	}
	owners := make([]Connection,0)
	owners = append(owners, ownerConn)
	ownerToSearch := filterConnections(connections, owners)
	for _, conn := range ownerToSearch {
		if conn.Kind != "" && conn.Name != "" {
			connections = GetRelatives(connections, level, conn.Kind, conn.Name, namespace)
		}
	}
	return connections
}

func findChildrenConnections(connections []Connection, level int, kind, instance, namespace string) []Connection {
	relatedKindList := findChildKinds(kind)
	childs := make([]Connection,0)
	for _, relKind := range relatedKindList {
		childResKindPlural, _, childResApiVersion, childResGroup := getKindAPIDetails(relKind)
		childRes := schema.GroupVersionResource{Group: childResGroup,
										 		Version: childResApiVersion,
										   		Resource: childResKindPlural}
		dynamicClient, err := getDynamicClient()

		children, err := dynamicClient.Resource(childRes).Namespace(namespace).List(context.TODO(),
																	   		 	    metav1.ListOptions{})
		if err != nil {
			children, err = dynamicClient.Resource(childRes).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				panic(err)
			}
		}
		for _, child := range children.Items {
			ownerKind, ownerInstance := findOwner(child)
			if ownerKind == kind && ownerInstance == instance {
				connection := Connection {
					Name: child.GetName(),
					Kind: relKind,
					Namespace: namespace,
				}
				childs = append(childs, connection)
			}
		}
	}
	//fmt.Printf("Connections:%v\n", connections)
	//fmt.Printf("Childs:%v\n", childs)
	childenToSearch := filterConnections(connections, childs)
	for _, conn := range childenToSearch {
		if conn.Kind != "" && conn.Name != "" {
			connections = GetRelatives(connections, level, conn.Kind, conn.Name, namespace)
		}
	}
	return connections
}

func searchOwnerGraph(connections, owners []Connection, level int) []Connection {
	level = level + 1
	unseenOwners := make([]Connection,0)
	for _, owner := range owners {
		present := false
		for _, connection := range connections {
			if connection.Name == owner.Name && connection.Kind == owner.Kind && connection.Namespace == owner.Namespace {
				present = true
			}
		}
		if !present {
			unseenOwners = append(unseenOwners, owner)
		}
	}
	for _, relative := range unseenOwners {
		kind := relative.Kind
		name := relative.Name
		namespace := relative.Namespace
		connections = GetRelatives(connections, level, kind, name, namespace)
	}
	return connections
} 

func searchNextLevel(connections []Connection, level int, relativeNames []string, targetKind, namespace string) ([]Connection) {
	for _, relativeName := range relativeNames {
		connections = GetRelatives(connections, level, targetKind, relativeName, namespace)
	}
	return connections
}

func findOwner(instanceObj unstructured.Unstructured) (string, string) {
	ownerKind := ""
	ownerName := ""
	ownerReference := instanceObj.GetOwnerReferences()
	if len(ownerReference) == 0 {
		return ownerKind, ownerName
	} else {
		owner := ownerReference[0]
		ownerKind = owner.Kind
		ownerName = owner.Name
		//ownerKind, ownerInstance = findOwner(oKind, oName, namespace)
		// Reached the root
		//if ownerKind == "" {
		//	ownerKind = oKind
		//}
		//if ownerInstance == "" {
		//	ownerInstance = oName
		//}
	}
	return ownerKind, ownerName
}

func getOwnerDetail(kind, instance, namespace string) (string, string) {
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
	ownerKind, ownerInstance = findOwner(*instanceObj)
	return ownerKind, ownerInstance
}

func searchAnnotations(kind, instance, namespace, annotationKey, annotationValue, targetKind, targetInstance string) ([]string, string) {
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
	//fmt.Printf("%v\n", lhsRes)
	lhsInstList, err := getObjects(instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}


	/*
	_, err = dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		return relativesNames, relDetail
	}*/
	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := getObjects(targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}

	/*
	rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})
	if err != nil {
		return relativesNames, relDetail
	}*/

	for _, instanceObj := range lhsInstList {
		//lhsContent := instanceObj.UnstructuredContent()
		lhsName := instanceObj.GetName()

		for _, unstructuredObj := range rhsInstList {
		//rhsContent := unstructuredObj.UnstructuredContent()
		//annotationMap, ok, _ := unstructured.NestedMap(rhsContent, "annotations")
			annotations := unstructuredObj.GetAnnotations()
		//fmt.Printf("AnnotationMap:%v\n", annotations)
			for key, value := range annotations {
				if instance == "*" {
					//fmt.Printf("Key:%s, AnnotationKey:%s, value:%s, lhsName:%s\n", key, annotationKey, value, lhsName)
					if key == annotationKey && value == lhsName {
						//rhsInstanceName := unstructuredObj.GetName()
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						relativesNames = append(relativesNames, lhsName)
					}
				} else {
					if key == annotationKey && value == instance {
						rhsInstanceName := unstructuredObj.GetName()
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						relativesNames = append(relativesNames, rhsInstanceName)
					}
				}
			}
		}
	}
	relDetail = annotationKey + "::" + annotationValue
	return relativesNames, relDetail
}

func searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]string, string) {
	//fmt.Printf("ABC\n")
	relativesNames := make([]string, 0)
	envNameValue := ""
	if lhs == "env" {
	//	fmt.Printf("DEF\n")
		relativesNames, envNameValue = searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind, targetInstance)
	} else {
	//	fmt.Printf("EFG\n")
		relativesNames, envNameValue = searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)		
	}
	return relativesNames, envNameValue
}

func searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]string, string) {
	relativesNames := make([]string, 0)
	propertyNameValue := ""

	dynamicClient, err := getDynamicClient()

	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	if err != nil {
		panic(err)
		return relativesNames, propertyNameValue
	}
	/*
	instanceObj, err := dynamicClient.Resource(lhsRes).Namespace(namespace).Get(context.TODO(),
																			 	instance,
																	   		 	metav1.GetOptions{})
	if err != nil {
		panic(err)
		return relativesNames, propertyNameValue
	}*/
	lhsInstList, err := getObjects(instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}

	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	//rhsInstList, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
	//																   metav1.ListOptions{})
	//if err != nil {
	//	return relativesNames, propertyNameValue
	//}
	rhsInstList, err := getObjects(targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}

	for _, instanceObj := range lhsInstList {
		lhsContent := instanceObj.UnstructuredContent()
		lhsName := instanceObj.GetName()
		//fmt.Printf("LHSContent:%v\n", lhsContent)
		fieldValue, found := findFieldValue(lhsContent, lhs)
		//fieldValue, found, err := unstructured.NestedString(lhsContent, "spec", lhs)
		//fmt.Printf("FieldValue:%s, found:%v, Error:%v", fieldValue, found, err)
		//if err != nil || !found {
		//	return relativesNames, propertyNameValue
		//}
		//fmt.Printf("ABC ABC: %v, %s", found, fieldValue)
		if found {
			for _, unstructuredObj := range rhsInstList {
				if rhs == "name" {
					rhsInstanceName := unstructuredObj.GetName()
					if fieldValue == rhsInstanceName {
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						/*present := false
						for _, relName := range relativesNames {
							if relName == rhsInstanceName {
								present = true
								break
							}
						}*/
						if instance == "*" {
							relativesNames = append(relativesNames, lhsName)
						} else {
							relativesNames = append(relativesNames, rhsInstanceName)
						}
						//relativesNames = append(relativesNames, rhsInstanceName)
						propertyNameValue = "Name:" + lhs + " " + "Value:" + fieldValue
						if found {
							break
						}
					}
				}
			}
		}
	}
	return relativesNames, propertyNameValue
}

func findFieldValue(lhsContent interface{}, specfield string) (string, bool) {
	fieldValue := ""
	found := false
	//fmt.Printf("lhsContent:%v\n", lhsContent)
	mapval, ok1 := lhsContent.(map[string]interface{})
	if ok1 {
		for key, value := range mapval {
			if found {
				break
			}
			//fmt.Printf("Key:%s, Value:%v\n", key, value)
			stringval, ok := value.(string)
			if ok {
				if key == specfield {
					//fmt.Printf("Field:%s Value:%s\n", key, stringval)
					fieldValue = stringval
					found = true
					break
				}
			} 
			mapval1, ok1 := value.(map[string]interface{})
			if ok1 {
				//fmt.Printf("Mapval1:%v\n", mapval1)
				fieldValue, found = findFieldValue(mapval1, specfield)
			}
			listval, ok2 := value.([]interface{})
			if ok2 {
				//fmt.Printf("Listval:%v\n", listval)
				for _, mapInList := range listval {
					mapInListMap, ok3 := mapInList.(map[string]interface{})
					if ok3 {
						fieldValue, found = findFieldValue(mapInListMap, specfield)
						if found {
							break
						}
						//return fieldValue, found
					}
					stringInListMap, ok4 := mapInList.(string)
					if ok4 {
						if key == specfield {
							//fmt.Printf("Field:%s Value:%s\n", key, stringInListMap)
							fieldValue = stringInListMap
							found = true
							break
						}
					}
				}
			}
		}
	}
	//fmt.Printf("FieldValue:%s\n", fieldValue)
	return fieldValue, found
}

func searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind, targetInstance string) ([]string, string) {
	relativesNames := make([]string, 0)
	envNameValue := ""
	//fmt.Printf("((kind:%s, instance:%s, targetKind:%s, targetInstance:%s))\n", kind, instance, targetKind, targetInstance)
	dynamicClient, err := getDynamicClient()
	if err != nil {
		return relativesNames, envNameValue
	}
	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	lhsInstList, err := getObjects(instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}

	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := getObjects(targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		panic(err)
	}
	/*rhsInstList := make([]*unstructured.Unstructured,0)
	if targetInstance == "*" {
		rhsInstances, err := dynamicClient.Resource(rhsRes).Namespace(namespace).List(context.TODO(),
																				   metav1.ListOptions{})
		if err != nil {
			return relativesNames, envNameValue
		}
		for _, rhsObj := range rhsInstances.Items {
			rhsObjCopy := rhsObj.DeepCopy()
			rhsInstList = append(rhsInstList, rhsObjCopy)
		}
	} else {
		rhsObj, err := dynamicClient.Resource(rhsRes).Namespace(namespace).Get(context.TODO(), targetInstance,
																		   	   metav1.GetOptions{})
		if err != nil {
			return relativesNames, envNameValue
		}
		rhsInstList = append(rhsInstList, rhsObj)
	} */

	//fmt.Printf("LHSList:%v\n", lhsInstList)
	//fmt.Printf("RHSList:%v\n", rhsInstList)
	for _, instanceObj := range lhsInstList {
		lhsContent := instanceObj.UnstructuredContent()
		lhsName := instanceObj.GetName()
		//jsonContent, _ := instanceObj.MarshalJSON()
		//fmt.Printf("JSON Content:%s\n", string(jsonContent))
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
						for _, unstructuredObj := range rhsInstList {
							//fmt.Printf(" Service name:%s\n", unstructuredObj.GetName())
							if rhs == "name" {
								rhsInstanceName := unstructuredObj.GetName()
								if envValue == rhsInstanceName {
									if instance == "*" {
										//fmt.Printf("LHS InstanceName:%s\n", lhsName)
										relativesNames = appendRelNames(relativesNames, lhsName)
									} else {
										//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
										relativesNames = appendRelNames(relativesNames, rhsInstanceName)
										envNameValue = "Name:" + envName + " " + "Value:" + envValue
									}
								}
							}
						}
					}
				}
			}
		}
	}
	//fmt.Printf("Spec Prop:%v\n", relativesNames)
	return relativesNames, envNameValue
}

func getObjects(instance, namespace string, res schema.GroupVersionResource, dynamicClient dynamic.Interface) ([]*unstructured.Unstructured, error) {
	lhsInstList := make([]*unstructured.Unstructured,0)
	var err error
	if instance == "*" {
		lhsInstances, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																		   		 	metav1.ListOptions{})
		if err != nil { // Check if this is a non-namespaced resource
			lhsInstances, err = dynamicClient.Resource(res).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				panic(err)
			}
		}
		for _, lhsObj := range lhsInstances.Items {
			//lhsName := lhsObj.GetName()
			//fmt.Printf("&&&%s\n", lhsName)
			lhsObjCopy := lhsObj.DeepCopy()
			lhsInstList = append(lhsInstList, lhsObjCopy)
		}
	} else {
		lhsObj, err := dynamicClient.Resource(res).Namespace(namespace).Get(context.TODO(), instance,
																		   	   metav1.GetOptions{})
		if err != nil { // Check if this is a non-namespaced resource
			lhsObj, err = dynamicClient.Resource(res).Get(context.TODO(), instance, metav1.GetOptions{})
			if err != nil {
				panic(err)
			}
		}
		lhsInstList = append(lhsInstList, lhsObj)
	}
	return lhsInstList, err
}

func parseRelationship(relString string) (string, string, string, []string) {
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
	if relType == "owner" {
		targetKind := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
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
	var found bool
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
	selectorMap, found, _ = unstructured.NestedStringMap(content, "spec", "selector")
	if !found {
		selectorMap, found, _ = unstructured.NestedStringMap(content, "spec", "selector", "matchLabels")
	}
	//fmt.Printf("SelectorMap:%s\n", selectorMap)
	return selectorMap
}

func searchSelectors(labelMap map[string]string, targetKind, namespace string) ([]string, string) {
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
		content := unstructuredObj.UnstructuredContent()
		selectorMap, found, _ := unstructured.NestedStringMap(content, "spec", "selector")
		if !found {
			selectorMap, found, _ = unstructured.NestedStringMap(content, "spec", "selector", "matchLabels")
		}
		//fmt.Printf("searchSelectors %s %v\n", unstructuredObj.GetName(), selectorMap)
		//fmt.Printf("searchSelectors %v\n", labelMap)
		match := subsetMatchMaps(selectorMap, labelMap)
		if match {
			instanceName := unstructuredObj.GetName()
			instanceNames = append(instanceNames, instanceName)
			for key, value := range selectorMap {
				relDetail = relDetail + key + ":" + value + " "
			}
		}
	}
	return instanceNames, relDetail
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