package discovery

import (
	"strings"
	"fmt"
	"context"
	"time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/dynamic"
)

func makeTimestamp() int64 {
    return time.Now().UnixNano() / int64(time.Millisecond)
}

func GetRelatives(visited [] Connection, level int, kind, instance, origkind, originstance, namespace, relType string) ([]Connection) {
	//_ = readKindCompositionFile(kind)
	/*if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return visited
	}*/

	//fmt.Printf("Node - Level: %d, Kind:%s, instance:%s origkind:%s, originstance:%s relType:%s\n", level, kind, instance, origkind, originstance, relType)
	if OutputFormat != "json" {
		_ = makeTimestamp()
		fmt.Printf("Discovering node - Level: %d, Kind:%s, instance:%s\n", level, kind, instance)
	} 
			inputInstance := Connection{
			Name: instance,
			Kind: kind,
			Namespace: namespace,
			RelationType: relType,
			Level: level, 
			Peer: &Connection{
				Name: "",
				Kind: "",
				Namespace: "",
			},
		}
		if kind != origkind && instance != originstance {
			peer := Connection{
				Name: originstance,
				Kind: origkind,
				Namespace: namespace,
			}
			inputInstance.Peer = &peer
		}
		inputInstanceList := make([]Connection,0)
		inputInstanceList = append(inputInstanceList, inputInstance)
		visited = appendCurrentLevelPeers(visited, inputInstanceList)
		visited = findRelatives(visited, level, kind, instance, origkind, originstance, namespace, relType)
	return visited
}

func findRelatives(visited []Connection, level int, kind, instance, origkind, originstance, namespace string, relType string) ([]Connection) {
	relStringList := relationshipMap[kind]
	visited = findDownstreamRelatives(visited, level, kind, instance, namespace, relStringList)
	//fmt.Printf("Kind:%s, relStringList:%v\n",kind, relStringList)

	relatedKindList := findRelatedKinds(kind)
	//fmt.Printf("Kind:%s, Related Kind List 1:%v\n", kind, relatedKindList)
	for _, relatedKind := range relatedKindList {
		relStringListRelated := relationshipMap[relatedKind]
		visited = findUpstreamRelatives(visited, level, relatedKind, kind, instance, namespace, relStringListRelated)
	}
	visited = findParentConnections(visited, level, kind, instance, namespace)
	visited = findChildrenConnections(visited, level, kind, instance, namespace)
	visited = findCompositionConnections(visited, level, kind, instance, namespace)
	return visited
}

func findDownstreamRelatives(visited []Connection, level int, kind, instance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		//fmt.Printf("Reltype:%s, lhs:%s, rhs:%s, TargetKindList:%v\n", relType, lhs, rhs, targetKindList)
		for _, targetKind := range targetKindList {
			if relType == relTypeLabel {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames, relDetail := searchLabels(level, kind, instance, selectorLabelMap, targetKind, namespace)
				//fmt.Printf("FDSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				visited = buildGraph(visited, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == relTypeSpecProperty {
				targetInstance := "*"
				relativesNames, relDetail, relTypeSpecific := searchSpecProperty(level, kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				relType = relTypeSpecific			
				//fmt.Printf("FDSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				visited = buildGraph(visited, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == relTypeAnnotation {
				targetInstance := "*"
				relativesNames, relDetail := searchAnnotations(level, kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
				visited = buildGraph(visited, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
		}
	}
	return visited
}

func findUpstreamRelatives(visited []Connection, level int, relatedKind, kind, instance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if targetKind == kind {
				if relType == relTypeLabel {
					labelMap := getLabels(kind, instance, namespace)
					relativesNames, relDetail := searchSelectors(level, relatedKind, labelMap, kind, instance, namespace)
					//fmt.Printf("FUSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
					visited = buildGraph(visited, level, kind, instance, relativesNames, relatedKind, namespace, relType, relDetail)
				}
				if relType == relTypeSpecProperty {
					targetInstance := "*"
					relativesNames, relDetail, relTypeSpecific := searchSpecProperty(level, relatedKind, targetInstance, namespace, lhs, rhs, kind, instance)
					//fmt.Printf("FUSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
					relType = relTypeSpecific
					visited = buildGraph(visited, level, kind, instance, relativesNames, relatedKind, namespace, relType, relDetail)
				}
				if relType == relTypeAnnotation {
					targetInstance := "*"
					relativesNames, relDetail := searchAnnotations(level, relatedKind, targetInstance, namespace, lhs, rhs, kind, instance)
					//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
					visited = buildGraph(visited, level, kind, instance, relativesNames, relatedKind, namespace, relType, relDetail)
				}
			}
		}
	}
	return visited
}

func buildGraph(visited []Connection, level int, kind, instance string, relativesNames []Connection, targetKind, namespace, relType, relDetail string) ([]Connection) {
	unseenRelatives, seenRelatives := filterConnections(visited, relativesNames)

	/*fmt.Printf("unseenRelatives:%v\n", unseenRelatives)
	fmt.Printf("seenRelatives:%v\n", seenRelatives)*/

	visited = appendCurrentLevelPeers(visited, relativesNames)
	visited = searchNextLevel(visited, level, unseenRelatives, kind, instance, targetKind, namespace, relType)

	for _, conn := range seenRelatives {
		TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)			
	}

	return visited
}

func filterConnections(allConnections []Connection, currentConnections []Connection) ([]Connection, []Connection) {
	connectionsToSearch := make([]Connection,0)
	existingConnections := make([]Connection,0)
	for _, currentConn := range currentConnections {
		found := false
		for _, conn := range allConnections {
			if currentConn.Name == conn.Name && 
			   currentConn.Kind == conn.Kind && 
			   currentConn.Namespace == conn.Namespace {
				found = true
				// Note that the connection object in allConnections might 
				// have empty Peer. This causes the o/p to show duplicate entries.
				// So copying the connection from the input.
				existingConnections = append(existingConnections, currentConn)
				break
			}
		}
		if !found {
			connectionsToSearch = append(connectionsToSearch, currentConn)
		}
	}
	return connectionsToSearch, existingConnections
}

func appendCurrentLevelPeers(connections, currentConnections []Connection) ([]Connection) {
	connections = appendConnections1(connections, currentConnections)
	return connections
}

func findCompositionConnections(visited []Connection, level int, kind, instance, namespace string) []Connection {
	if _, ok := crdcompositionMap[kind]; ok {

	composition := TotalClusterCompositions.GetCompositions(kind, instance, namespace)
	childrenConnections := make([]Connection, 0)

	for _, comp := range composition {
		if comp.Level == 1 {
			children := comp.Children
			for _, child := range children {
				childKind := child.Kind
				childInstance := child.Name
				childNamespace := child.Namespace

				childConn := Connection{
					Name: childInstance,
					Kind: childKind,
					Namespace: childNamespace,
					RelationType: relTypeOwnerReference,
					Peer: &Connection{
						Kind: kind,
						Name: instance,
						Namespace: namespace,
					},
					Level: level,
				}
				childrenConnections = append(childrenConnections, childConn)
			}
		}
	}

	childrenToSearch, seenRelatives := filterConnections(visited, childrenConnections)

	if len(childrenToSearch) > 0 {
		level = level + 1
		for _, conn := range childrenToSearch {
				relType := relTypeOwnerReference
				//fmt.Printf("Conn.Kind:%s Conn.Name:%s kind:%s instance:%s\n", conn.Kind, conn.Name, kind, instance)
				TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
				visited = GetRelatives(visited, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
		}
	}

	for _, conn := range seenRelatives {
		//fmt.Printf("4\n")
		TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
	}
	}
	return visited
}

func findParentConnections(visited []Connection, level int, kind, instance, namespace string) []Connection {
	ownerKind, ownerInstance := getOwnerDetail(kind, instance, namespace)
	peer := Connection{
				Kind: kind,
				Name: instance,
				Namespace: namespace,
				RelationType: relTypeOwnerReference,
	}
	//fmt.Printf("Kind:%s Instance:%s OwnerKind:%s OwnerInstance:%s\n", kind, instance, ownerKind, ownerInstance)
	if ownerKind != "" && ownerInstance != "" {
		ownerConn := Connection{
			Name: ownerInstance,
			Kind: ownerKind,
			Namespace: namespace,
			RelationType: relTypeOwnerReference,
			Level: level,
		}
		if ownerKind != kind {
			//fmt.Printf("ABC ownerKind:%s kind:%s ownerInstance:%s instance:%s\n", ownerKind, kind, ownerInstance, instance)
			ownerConn.Peer = &peer
		} else {
			//fmt.Printf("DEF ownerKind:%s kind:%s ownerInstance:%s instance:%s\n", ownerKind, kind, ownerInstance, instance)
			ownerConn.Peer = &Connection{
				Name: "",
				Kind: "",
				Namespace: "",
			}
		}

		owners := make([]Connection,0)
		owners = append(owners, ownerConn)
		ownerToSearch, seenRelatives := filterConnections(visited, owners)

		if len(ownerToSearch) > 0 {
			level = level + 1
			for _, conn := range ownerToSearch {
				relType := relTypeOwnerReference
				//fmt.Printf("ABC:%v\n", conn)
				TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
				visited = GetRelatives(visited, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
			}
		}
		for _, conn := range seenRelatives {
			//fmt.Printf("4 conn:%v\n", conn)
			TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
		}
	}
	return visited
}

func findChildrenConnections(visited []Connection, level int, kind, instance, namespace string) []Connection {
	relatedKindList := findChildKinds(kind)
	childs := make([]Connection,0)
	peer := Connection{
				Kind: kind,
				Name: instance,
				Namespace: namespace,
				RelationType: relTypeOwnerReference,
	}
	for _, relKind := range relatedKindList {
		childResKindPlural, _, childResApiVersion, childResGroup := getKindAPIDetails(relKind)
		childRes := schema.GroupVersionResource{Group: childResGroup,
										 		Version: childResApiVersion,
										   		Resource: childResKindPlural}
		//dynamicClient, err := getDynamicClient()

		children, err := getKubeObjectList(relKind, namespace, childRes)
		if err != nil {
				return visited
		}
		/*
		children, err := dynamicClient.Resource(childRes).Namespace(namespace).List(context.TODO(),
																	   		 	    metav1.ListOptions{})

		if err != nil {
			children, err = dynamicClient.Resource(childRes).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return visited
			}
		}*/
		for _, child := range children.Items {
			ownerKind, ownerInstance := findOwner(child)
			if ownerKind == kind && ownerInstance == instance {
				connection := Connection {
					Name: child.GetName(),
					Kind: relKind,
					Namespace: namespace,
					RelationType: relTypeOwnerReference,
					Level: level,
					Peer: &peer,
				}
				childs = append(childs, connection)
			}
		}
	}
	//fmt.Printf("Connections:%v\n", visited)
	//fmt.Printf("Childs:%v\n", childs)
	childrenToSearch, seenRelatives := filterConnections(visited, childs)
	//fmt.Printf("ChildrenToSearch:%v\n", childrenToSearch)

	if len(childrenToSearch) > 0 {
		level = level + 1
		for _, conn := range childrenToSearch {
			relType := relTypeOwnerReference
			if conn.Kind != "" && conn.Name != "" {
				//fmt.Printf("ABC:%v\n", conn)
				TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
				visited = GetRelatives(visited, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
			}
		}
	}

	for _, conn := range seenRelatives {
		TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
	}
	return visited
}

func searchNextLevel(visited []Connection, level int, relativeNames []Connection, kind, instance, targetKind, namespace, relType string) ([]Connection) {
	level = level + 1
	for _, relative := range relativeNames {
		relativeName := relative.Name
		TotalClusterConnections = AppendConnections(TotalClusterConnections, relative)
		visited = GetRelatives(visited, level, targetKind, relativeName, kind, instance, namespace, relType)
	}
	return visited
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
	/*dynamicClient, err := getDynamicClient()
	if err != nil {
		return ownerKind, ownerInstance
	}
	instanceObj, err := dynamicClient.Resource(ownerRes).Namespace(namespace).Get(context.TODO(),
																			 	  instance,
															   		 	  metav1.GetOptions{})
	*/
	instanceObj, err := getKubeObject(kind, instance, namespace, ownerRes)
	if err != nil {
		return ownerKind, ownerInstance
	}
	ownerKind, ownerInstance = findOwner(instanceObj)
	return ownerKind, ownerInstance
}

func searchAnnotations(level int, kind, instance, namespace, annotationKey, annotationValue, targetKind, targetInstance string) ([]Connection, string) {
	relativesNames := make([]Connection, 0)
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
	lhsInstList, err := getObjects(kind, instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		return relativesNames, relDetail
	}

	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := getObjects(targetKind, targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		return relativesNames, relDetail
	}

	for _, instanceObj := range lhsInstList {
		//lhsContent := instanceObj.UnstructuredContent()
		lhsName := instanceObj.GetName()

		for _, unstructuredObj := range rhsInstList {
			//rhsName := unstructuredObj.GetName()
			//fmt.Printf("RHS Name:%s\n", rhsName)
		//rhsContent := unstructuredObj.UnstructuredContent()
		//annotationMap, ok, _ := unstructured.NestedMap(rhsContent, "annotations")
			annotations := unstructuredObj.GetAnnotations()
		//fmt.Printf("AnnotationMap:%v\n", annotations)
			for key, value := range annotations {
				//fmt.Printf("Key:%s, AnnotationKey:%s, value:%s, lhsName:%s\n", key, annotationKey, value, lhsName)
				if instance == "*" {
					if key == annotationKey && (value == lhsName || strings.Contains(value, lhsName)) {
						//rhsInstanceName := unstructuredObj.GetName()
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						relDetail = annotationKey + "::" + annotationValue
						conn := Connection{
							Level: level,
							Name: lhsName,
							Kind: kind,
							Namespace: namespace,
							RelationDetails: relDetail,
							RelationType: relTypeAnnotation,
							Peer: &Connection{
								Name: targetInstance,
								Kind: targetKind,
								Namespace: namespace,
							},
						}
						relativesNames = append(relativesNames, conn)
					}
				} else {
					//fmt.Printf("Value:%s\n", value)
					//fmt.Printf("Instance:%s\n", instance)
					if key == annotationKey && (value == instance || strings.Contains(value, instance)) {
						rhsInstanceName := unstructuredObj.GetName()
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						relDetail = annotationKey + "::" + annotationValue
						conn := Connection{
							Level: level,
							Name: rhsInstanceName,
							Kind: targetKind,
							Namespace: namespace,
							RelationDetails: relDetail,
							RelationType: relTypeAnnotation,
							Peer: &Connection{
								Name: instance,
								Kind: kind,
								Namespace: namespace,
							},
						}
						relativesNames = append(relativesNames, conn)
					}
				}
			}
		}
	}
	return relativesNames, relDetail
}

func searchSpecProperty(level int, kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]Connection, string, string) {
	relativesNames := make([]Connection, 0)
	envNameValue := ""
	relTypeSpecific := ""
	if lhs == "env" {
		relativesNames, envNameValue = searchSpecPropertyEnv(level, kind, instance, namespace, rhs, targetKind, targetInstance)
		relTypeSpecific = relTypeEnvvariable
	} else {
		relativesNames, envNameValue = searchSpecPropertyField(level, kind, instance, namespace, lhs, rhs, targetKind, targetInstance)		
		relTypeSpecific = relTypeSpecProperty
	}
	return relativesNames, envNameValue, relTypeSpecific
}

func searchSpecPropertyField(level int, kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]Connection, string) {
	relativesNames := make([]Connection, 0)
	propertyNameValue := ""

	dynamicClient, err := getDynamicClient()

	lhsResKindPlural, _, lhsResApiVersion, lhsResGroup := getKindAPIDetails(kind)
	lhsRes := schema.GroupVersionResource{Group: lhsResGroup,
									   Version: lhsResApiVersion,
									   Resource: lhsResKindPlural}
	if err != nil {
		return relativesNames, propertyNameValue
	}
	lhsInstList, err := getObjects(kind, instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		return relativesNames, propertyNameValue
	}

	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := getObjects(targetKind, targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		return relativesNames, propertyNameValue
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
						var connName, connKind string
						var peerName, peerKind string
						if instance == "*" {
							connName = lhsName
							connKind = kind
							peerName = rhsInstanceName
							peerKind = targetKind
							//relativesNames = append(relativesNames, lhsName)
						} else {
							connName = rhsInstanceName
							connKind = targetKind
							peerName = lhsName
							peerKind = kind
							//relativesNames = append(relativesNames, rhsInstanceName)
						}
						propertyNameValue = "Name:" + lhs + " " + "Value:" + fieldValue
						conn := Connection{
							Level: level,
							Name: connName,
							Kind: connKind,
							Namespace: namespace,
							RelationDetails: propertyNameValue,
							RelationType: relTypeSpecProperty,
							Peer: &Connection{
								Name: peerName,
								Kind: peerKind,
								Namespace: namespace,
							},
						}
						relativesNames = append(relativesNames, conn)
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

	// Handle default field values such as Pod.spec.serviceAccountName
	if !found {
		if specfield == "serviceAccountName" {
			fieldValue = "default"
			found = true
		}
	}
	return fieldValue, found
}

func searchSpecPropertyEnv(level int, kind, instance, namespace, rhs, targetKind, targetInstance string) ([]Connection, string) {
	relativesNames := make([]Connection, 0)
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
	lhsInstList, err := getObjects(kind, instance, namespace, lhsRes, dynamicClient)
	if err != nil {
		return relativesNames, envNameValue
	}

	rhsResKindPlural, _, rhsResApiVersion, rhsResGroup := getKindAPIDetails(targetKind)
	rhsRes := schema.GroupVersionResource{Group: rhsResGroup,
									   Version: rhsResApiVersion,
									   Resource: rhsResKindPlural}
	rhsInstList, err := getObjects(targetKind, targetInstance, namespace, rhsRes, dynamicClient)
	if err != nil {
		return relativesNames, envNameValue
	}

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
									var connName, connKind string
									var peerName, peerKind string
									if instance == "*" {
										//fmt.Printf("LHS InstanceName:%s\n", lhsName)
										connName = lhsName
										connKind = kind
										peerName = rhsInstanceName
										peerKind = targetKind
									} else {
										//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
										connName = rhsInstanceName
										connKind = targetKind
										peerName = lhsName
										peerKind = kind
										envNameValue = "Name:" + envName + " " + "Value:" + envValue
									}
									conn := Connection{
										Level: level,
										Name: connName,
										Kind: connKind,
										Namespace: namespace,
										RelationDetails: envNameValue,
										RelationType: relTypeEnvvariable,
										Peer: &Connection{
											Name: peerName,
											Kind: peerKind,
											Namespace: namespace,
										},
									}
									connList := make([]Connection,0)
									connList = append(connList,conn)
									relativesNames = appendConnections1(relativesNames, connList)
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

func getObjects(kind, instance, namespace string, res schema.GroupVersionResource, dynamicClient dynamic.Interface) ([]*unstructured.Unstructured, error) {
	lhsInstList := make([]*unstructured.Unstructured,0)
	var err error
	if instance == "*" {
		/*
		lhsInstances, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																		   		 	metav1.ListOptions{})
		if err != nil { // Check if this is a non-namespaced resource
			lhsInstances, err = dynamicClient.Resource(res).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				//panic(err)
				return lhsInstList, err
			}
		}*/
		
		lhsInstances, err := getKubeObjectList(kind, namespace, res)
		if err != nil {
			return lhsInstList, err
		}

		for _, lhsObj := range lhsInstances.Items {
			//lhsName := lhsObj.GetName()
			//fmt.Printf("&&&%s\n", lhsName)
			lhsObjCopy := lhsObj.DeepCopy()
			lhsInstList = append(lhsInstList, lhsObjCopy)
		}
	} else {
		/*
		lhsObj, err := dynamicClient.Resource(res).Namespace(namespace).Get(context.TODO(), instance,
																		   	   metav1.GetOptions{})
		if err != nil { // Check if this is a non-namespaced resource
			lhsObj, err = dynamicClient.Resource(res).Get(context.TODO(), instance, metav1.GetOptions{})
			if err != nil {
				//panic(err)
				return lhsInstList, err
			}
		}*/

		lhsObj, err1 := getKubeObject(kind, instance, namespace, res)
		if err1 != nil {
			err = err1
		} else {
			lhsInstList = append(lhsInstList, &lhsObj)
		}
	}
	return lhsInstList, err
}

func parseRelationship(relString string) (string, string, string, []string) {
	targetKindList := make([]string, 0)
	parts := strings.Split(relString, ",")
	relType := strings.TrimSpace(parts[0])
	var lhs, rhs string
	if relType == relTypeLabel {
		targetKind := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhs = strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		targetKindList = append(targetKindList, targetKind)
	}
	if relType == relTypeSpecProperty {
		targetKindString := strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		targetKindStringParts := strings.Split(targetKindString, ".")
		targetKind := targetKindStringParts[0]
		targetKindList = append(targetKindList, targetKind)
		rhs = targetKindStringParts[len(targetKindStringParts)-1]

		lhsString := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhsStringParts := strings.Split(lhsString, ".")
		lhs = lhsStringParts[len(lhsStringParts)-1]
	}
	if relType == relTypeAnnotation {
		targetKind := strings.Split(strings.TrimSpace(parts[1]), ":")[1]
		lhs = strings.Split(strings.TrimSpace(parts[2]), ":")[1]
		rhsparts := strings.Split(strings.TrimSpace(parts[3]), ":")
		if len(rhsparts) == 2 { // value:name
			rhs = rhsparts[1]
		} else if len(rhsparts) == 3 { //value:[{name:INSTANCE.metadata.name}]
			rhs = rhsparts[2]
			rhs = strings.ReplaceAll(rhs, "}", "")
			rhs = strings.ReplaceAll(rhs, "]", "")
		}
		targetKindList = append(targetKindList, targetKind)
	}
	if relType == relTypeOwnerReference {
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
		//fmt.Printf(err.Error())
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

	//instanceObj, err := getKubeObject(kind, instance, namespace, res)
	if err != nil {
		//fmt.Printf("ABC\n")
		//fmt.Printf(err.Error())
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
		//fmt.Printf(err.Error())
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
	
	//instanceObj, err := getKubeObject(kind, instance, namespace, res)

	if err != nil {
		//fmt.Printf(err.Error())
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

func searchSelectors(level int, lhsKind string, labelMap map[string]string, rhsKind, rhsInstance, namespace string) ([]Connection, string) {
	instanceNames := make([]Connection, 0)
	relDetail := ""
	/*dynamicClient, err := getDynamicClient()
	if err != nil {
		return instanceNames, relDetail
	}*/
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(lhsKind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}

	list, err := getKubeObjectList(lhsKind, namespace, res)
		
	/*list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{}) */
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
		match := false
		if len(selectorMap) > 0 {
			match = subsetMatchMaps(selectorMap, labelMap)
		} 
		if match {
			for key, value := range selectorMap {
				relDetail = relDetail + key + ":" + value + " "
			}
			instanceName := Connection{
				Level: level,
				Name: unstructuredObj.GetName(),
				Kind: lhsKind,
				Namespace: namespace,
				RelationDetails: relDetail,
				RelationType: relTypeLabel,
				Peer: &Connection{
					Name: rhsInstance,
					Kind: rhsKind,
					Namespace: namespace,
				},
			}
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames, relDetail
}

func searchNameInLabels(name string, label map[string]string) bool {
	if len(label) == 0 {
		return false
	}
	for _, element := range label {
		if element == name {
			return true
		}
	}
	return false
}

func searchLabels(level int, sourceKind, sourceInstance string, labelMap map[string]string, targetKind, namespace string) ([]Connection, string) {
	instanceNames := make([]Connection, 0)
	relDetail := ""
	/*dynamicClient, err := getDynamicClient()
	if err != nil {
		return instanceNames, relDetail
	}*/
	resourceKindPlural, _, resourceApiVersion, resourceGroup := getKindAPIDetails(targetKind)
	res := schema.GroupVersionResource{Group: resourceGroup,
									   Version: resourceApiVersion,
									   Resource: resourceKindPlural}

	list, err := getKubeObjectList(targetKind, namespace, res)

	/*list, err := dynamicClient.Resource(res).Namespace(namespace).List(context.TODO(),
																	   metav1.ListOptions{})*/
	if err != nil {
		return instanceNames, relDetail
	}
	for _, unstructuredObj := range list.Items {
		unstructuredObjLabelMap := unstructuredObj.GetLabels()
		match := false
		if len(labelMap) > 0 {
			match = subsetMatchMaps(labelMap, unstructuredObjLabelMap)
		} else {
			match = searchNameInLabels(sourceInstance, unstructuredObjLabelMap)
		}
		if match {
			for key, value := range labelMap {
				relDetail = relDetail + key + ":" + value + " "
			}
			instanceName := Connection{
				Level: level,
				Name: unstructuredObj.GetName(),
				Kind: targetKind,
				Namespace: namespace,
				RelationDetails: relDetail,
				RelationType: relTypeLabel,
				Peer: &Connection{
					Name: sourceInstance,
					Kind: sourceKind,
					Namespace: namespace,
				},
			}
			instanceNames = append(instanceNames, instanceName)
		}
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