package discovery

import (
	"strings"
	"fmt"
	"context"
	"os"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/dynamic"
)

func GetRelatives(connections [] Connection, level int, kind, instance, origkind, originstance, namespace, relType string) ([]Connection) {
	err := readKindCompositionFile(kind)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return connections
	}
	exists := checkExistence(kind, instance, namespace)

	if exists {
	//fmt.Printf("Discovering connections - Level: %d, Kind:%s, instance:%s origkind:%s, originstance:%s\n", level, kind, instance, origkind, originstance)
		fmt.Printf("Discovering connections - Level: %d, Kind:%s, instance:%s\n", level, kind, instance)	
		if kind != origkind && instance != originstance {
			node := Connection{
				Level: level,
				Kind: kind,
				Name: instance,
				Namespace: namespace,
				RelationType: relType,
				Peer: &Connection{
					Kind: origkind,
					Name: originstance,
					Namespace: namespace,
				},
			}
			TotalClusterConnections = AppendConnections(TotalClusterConnections, node)
		}

		relatedKindList := findRelatedKinds(kind)
		//fmt.Printf("Kind:%s, Related Kind List 1:%v\n", kind, relatedKindList)
		connections = findRelatives(connections, level, kind, instance, origkind, originstance, namespace, relatedKindList, relType)
	} else {
		fmt.Printf("Resource %s of kind %s in namespace %s does not exist.\n", instance, kind, namespace)
		os.Exit(1)
	}
	return connections
}

func checkExistence(kind, instance, namespace string) bool {
	if instance == "" {
		return false
	}
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
			//panic(err1)
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
			if relType == relTypeOwnerReference {
				for _, tk := range targetKindList {
					childKinds = append(childKinds, tk)
				}
			}
		}
	}
	return childKinds
}

func findRelatives(connections []Connection, level int, kind, instance, origkind, originstance, namespace string, relatedKindList []string, relType string) ([]Connection) {
	// Put self in the list of relations so that we don't traverse back to self.
	
	inputInstance := Connection{
		Name: instance,
		Kind: kind,
		Namespace: namespace,
		RelationType: relType,
		Peer: &Connection{
			Name: originstance,
			Kind: origkind,
			Namespace: namespace,
		},
		Level: level, 
	}
	relativesNames := make([]Connection,0)
	relativesNames = append(relativesNames, inputInstance)

	unseenRelatives := filterConnections(connections, relativesNames)
	if len(unseenRelatives) == 0 {
		TotalClusterConnections = AppendConnections(TotalClusterConnections, inputInstance)
	}

	currentConnections := prepare(level, kind, instance, connections, unseenRelatives, kind, namespace, relType, "")
	connections = appendCurrentLevelPeers(connections, currentConnections, level)

	level = level + 1
	relStringList := relationshipMap[kind]
	connections = findDownstreamRelatives(connections, level, kind, instance, namespace, relStringList)
	for _, relatedKind := range relatedKindList {
		relStringList = relationshipMap[relatedKind]
		connections = findUpstreamRelatives(connections, level, relatedKind, instance, namespace, relStringList)
	}
	connections = findParentConnections(connections, level, kind, instance, namespace)
	connections = findChildrenConnections(connections, level, kind, instance, namespace)
	connections = findCompositionConnections(connections, level, kind, instance, namespace)
	connections = setPeers(connections, kind, instance, origkind, originstance, namespace, level)
	return connections
}

func findDownstreamRelatives(connections []Connection, level int, kind, instance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if relType == relTypeLabel {
				selectorLabelMap := getSelectorLabels(kind, instance, namespace)
				relativesNames, relDetail := searchLabels(selectorLabelMap, targetKind, namespace)
				//fmt.Printf("FDSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == relTypeSpecProperty {
				targetInstance := "*"
				relativesNames, relDetail, relTypeSpecific := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				relType = relTypeSpecific			
				//fmt.Printf("FDSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
			if relType == relTypeAnnotation {
				targetInstance := "*"
				relativesNames, relDetail := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
				connections = buildGraph(connections, level, kind, instance, relativesNames, targetKind, namespace, relType, relDetail)
			}
		}
	}
	return connections
}

func findUpstreamRelatives(connections []Connection, level int, kind, targetInstance, namespace string, relStringList []string) ([]Connection) {
	for _, relString := range relStringList {
		relType, lhs, rhs, targetKindList := parseRelationship(relString)
		for _, targetKind := range targetKindList {
			if relType == relTypeLabel {
				labelMap := getLabels(targetKind, targetInstance, namespace)
				relativesNames, relDetail := searchSelectors(labelMap, kind, namespace)
				//fmt.Printf("FUSR label - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				connections = buildGraph(connections, level, targetKind, targetInstance, relativesNames, kind, namespace, relType, relDetail)
			}
			if relType == relTypeSpecProperty {
				instance := "*"
				relativesNames, relDetail, relTypeSpecific := searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FUSR Spec - Relnames:%v Relatives:%v\n", relativesNames, relatives)
				relType = relTypeSpecific
				connections = buildGraph(connections, level, targetKind, targetInstance, relativesNames, kind, namespace, relType, relDetail)
			}
			if relType == relTypeAnnotation {
				instance := "*"
				relativesNames, relDetail := searchAnnotations(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)
				//fmt.Printf("FDSR Annotation:%v\n", relativesNames)				
				connections = buildGraph(connections, level, targetKind, targetInstance, relativesNames, kind, namespace, relType, relDetail)
			}
		}
	}
	return connections
}

func buildGraph(connections []Connection, level int, kind, instance string, relativesNames []Connection, targetKind, namespace, relType, relDetail string) ([]Connection) {
	currentConnections := prepare(level, kind, instance, connections, relativesNames, targetKind, namespace, relType, relDetail)
	unseenRelatives := filterConnections(connections, relativesNames)
	//fmt.Printf("UnseenRelatives:%v\n", unseenRelatives)
	connections = appendCurrentLevelPeers(connections, currentConnections, level)
	nextLevelConnections := searchNextLevel(connections, level, unseenRelatives, kind, instance, targetKind, namespace, relType)
	//fmt.Printf("Connections:%v\n", connections)
	//fmt.Printf("NextLevelConn:%v\n", nextLevelConnections)
	connections = appendNextLevelPeers(connections, nextLevelConnections)
	return connections
}

func setPeers(connections []Connection, kind, instance, origkind, originstance, namespace string, origlevel int) []Connection {
	for _, conn := range connections {
		if conn.Name == instance && conn.Kind == kind {
			if conn.Peer == nil {
				conn.Peer = &Connection{
				Kind: origkind,
				Name: originstance,
				Namespace: namespace,
			   }
			} 
		}
	}
	return connections
}

func searchConnection(connections []Connection, kind, instance, namespace string) Connection {
	var foundconn Connection
	for _, conn := range connections {
		if conn.Kind == kind && conn.Name == instance && conn.Namespace == namespace {
			conn = foundconn
			break
		}
	}
	return foundconn
}

func getLevel(connections []Connection, conn Connection) int {
	var level int
	for _, conni := range connections {
		if conni.Name == conn.Name && conni.Kind == conn.Kind && conn.Namespace == conni.Namespace {
			level = conn.Level
			break
		}
	}
	return level
}

func deepCopy(input Connection) Connection {
	var output Connection
	output.Peer = input.Peer
	output.Name = input.Name
	output.Kind = input.Kind
	output.Namespace = input.Namespace
	output.Level = input.Level
	output.Owner = input.Owner
	output.RelationType = input.RelationType
	output.RelationDetails = input.RelationDetails
	output.OwnerKind = input.OwnerKind
	output.OwnerName = input.OwnerName
	return output
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
				}
				if found {
					break
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
	connections = appendConnections1(connections, currentConnections)
	return connections
}

func appendNextLevelPeers(connections, nextLevelConnections []Connection) ([]Connection) {
	for _, nconnect := range nextLevelConnections {
		present := false
		for _, connect := range connections {
			if compareConnections(nconnect, connect) {
				present = true
				break
			}
		}
		if !present {
			connections = append(connections, nconnect)
		}
	}
	return connections
}

func prepare(level int, kind, instance string, connections, relativeNames []Connection, targetKind, namespace, relType, relDetail string) ([]Connection) {
	preparedConnections := make([]Connection,0)
	for _, relative := range relativeNames {
		relativeName := relative.Name
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
		if kind != targetKind && instance != relativeName {
			connection.Peer = &Connection{
				Name: instance, 
				Kind: kind,
				Namespace: namespace,
				Level: level + 1,
			}
		}
		preparedConnections = append(preparedConnections, connection)
	}
	return preparedConnections
}

func findCompositionConnections(connections []Connection, level int, kind, instance, namespace string) []Connection {
	
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
						Namespace: namespace, // should be same as childNamespace
						Level: level + 1,
					},
					Level: level + 1,
				}
				childrenConnections = append(childrenConnections, childConn)
			}
		}
	}

	//childrenToSearch := filterConnections(connections, childrenConnections)
	childrenToSearch := childrenConnections
	for _, conn := range childrenToSearch {
		relType := relTypeOwnerReference
		connections = GetRelatives(connections, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
	}
	}
	return connections
}


func findParentConnections(connections []Connection, level int, kind, instance, namespace string) []Connection {
	ownerKind, ownerInstance := getOwnerDetail(kind, instance, namespace)
	peer := Connection{
				Kind: kind,
				Name: instance,
				Namespace: namespace,
				RelationType: relTypeOwnerReference,
	}
	if ownerKind != "" && ownerInstance != "" {
		ownerConn := Connection{
			Name: ownerInstance,
			Kind: ownerKind,
			Namespace: namespace,
			RelationType: relTypeOwnerReference,
			Peer: &peer,
			Level: level + 1,
		}
		owners := make([]Connection,0)
		owners = append(owners, ownerConn)
		ownerToSearch := filterConnections(connections, owners)
		//ownerToSearch := owners
		//fmt.Printf("Owners:%v\n", ownerToSearch)
		if len(ownerToSearch) > 0 {
			for _, conn := range ownerToSearch {
				relType := relTypeOwnerReference
				connections = GetRelatives(connections, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
			}
		} else {
			for _, conn := range owners {
				TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
			}
		}
	}
	return connections
}

func findChildrenConnections(connections []Connection, level int, kind, instance, namespace string) []Connection {
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
					RelationType: relTypeOwnerReference,
					Level: level + 1,
					Peer: &peer,
				}
				childs = append(childs, connection)
			}
		}
	}
	//fmt.Printf("Connections:%v\n", connections)
	//fmt.Printf("Childs:%v\n", childs)
	childrenToSearch := filterConnections(connections, childs)
	//childrenToSearch := childs
	//fmt.Printf("ChildrenToSearch:%v\n", childrenToSearch)
	if len(childrenToSearch) > 0 {
		for _, conn := range childrenToSearch {
			relType := relTypeOwnerReference
			if conn.Kind != "" && conn.Name != "" {
				connections = GetRelatives(connections, level, conn.Kind, conn.Name, kind, instance, namespace, relType)
			}
		}
	} else {
		//fmt.Printf("Child:%v\n", childs)
		//fmt.Printf("TotalClusterConnections:%v\n", TotalClusterConnections)
		for _, conn := range childs {
			TotalClusterConnections = AppendConnections(TotalClusterConnections, conn)
		}
		//fmt.Printf("TotalClusterConnections1:%v\n", TotalClusterConnections)
	}
	return connections
}

func searchNextLevel(connections []Connection, level int, relativeNames []Connection, kind, instance, targetKind, namespace, relType string) ([]Connection) {
	for _, relative := range relativeNames {
		relativeName := relative.Name
		connections = GetRelatives(connections, level, targetKind, relativeName, kind, instance, namespace, relType)
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

func searchAnnotations(kind, instance, namespace, annotationKey, annotationValue, targetKind, targetInstance string) ([]Connection, string) {
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
						//relativesNames = append(relativesNames, lhsName)
						relDetail = annotationKey + "::" + annotationValue
						conn := Connection{
							Name: lhsName,
							Kind: kind,
							Namespace: namespace,
							RelationDetails: relDetail,
							RelationType: relTypeAnnotation,
						}
						relativesNames = append(relativesNames, conn)
					}
				} else {
					if key == annotationKey && value == instance {
						rhsInstanceName := unstructuredObj.GetName()
						//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
						//relativesNames = append(relativesNames, rhsInstanceName)
						relDetail = annotationKey + "::" + annotationValue
						conn := Connection{
							Name: rhsInstanceName,
							Kind: targetKind,
							Namespace: namespace,
							RelationDetails: relDetail,
							RelationType: relTypeAnnotation,
						}
						relativesNames = append(relativesNames, conn)
					}
				}
			}
		}
	}
	return relativesNames, relDetail
}

func searchSpecProperty(kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]Connection, string, string) {
	relativesNames := make([]Connection, 0)
	envNameValue := ""
	relTypeSpecific := ""
	if lhs == "env" {
		relativesNames, envNameValue = searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind, targetInstance)
		relTypeSpecific = relTypeEnvvariable
	} else {
		relativesNames, envNameValue = searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind, targetInstance)		
		relTypeSpecific = relTypeSpecProperty
	}
	return relativesNames, envNameValue, relTypeSpecific
}

func searchSpecPropertyField(kind, instance, namespace, lhs, rhs, targetKind, targetInstance string) ([]Connection, string) {
	relativesNames := make([]Connection, 0)
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
						if instance == "*" {
							connName = lhsName
							connKind = kind
							//relativesNames = append(relativesNames, lhsName)
						} else {
							connName = rhsInstanceName
							connKind = targetKind
							//relativesNames = append(relativesNames, rhsInstanceName)
						}
						propertyNameValue = "Name:" + lhs + " " + "Value:" + fieldValue
						conn := Connection{
							Name: connName,
							Kind: connKind,
							Namespace: namespace,
							RelationDetails: propertyNameValue,
							RelationType: relTypeSpecProperty,
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
	return fieldValue, found
}

func searchSpecPropertyEnv(kind, instance, namespace, rhs, targetKind, targetInstance string) ([]Connection, string) {
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
									if instance == "*" {
										//fmt.Printf("LHS InstanceName:%s\n", lhsName)
										connName = lhsName
										connKind = kind
									} else {
										//fmt.Printf("RHS InstanceName:%s\n", rhsInstanceName)
										connName = rhsInstanceName
										connKind = targetKind
										envNameValue = "Name:" + envName + " " + "Value:" + envValue
									}
									conn := Connection{
										Name: connName,
										Kind: connKind,
										Namespace: namespace,
										RelationDetails: envNameValue,
										RelationType: relTypeEnvvariable,
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
		rhs = strings.Split(strings.TrimSpace(parts[3]), ":")[1]
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

func searchSelectors(labelMap map[string]string, targetKind, namespace string) ([]Connection, string) {
	instanceNames := make([]Connection, 0)
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
			for key, value := range selectorMap {
				relDetail = relDetail + key + ":" + value + " "
			}
			instanceName := Connection{
				Name: unstructuredObj.GetName(),
				Kind: targetKind,
				Namespace: namespace,
				RelationDetails: relDetail,
				RelationType: relTypeLabel,
			}
			instanceNames = append(instanceNames, instanceName)
		}
	}
	return instanceNames, relDetail
}

func searchLabels(labelMap map[string]string, targetKind, namespace string) ([]Connection, string) {
	instanceNames := make([]Connection, 0)
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
			for key, value := range labelMap {
				relDetail = relDetail + key + ":" + value + " "
			}
			instanceName := Connection{
				Name: unstructuredObj.GetName(),
				Kind: targetKind,
				Namespace: namespace,
				RelationDetails: relDetail,
				RelationType: relTypeLabel,
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