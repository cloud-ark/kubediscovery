package discovery

import (
	"os"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"path/filepath"
	"github.com/coreos/etcd/client"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	cfg *rest.Config
	err error
)

func init() {
	cfg, err = buildConfig()
	if err != nil {
		panic(err.Error())
	}
}

func buildConfig() (*rest.Config, error) {
	if home := homeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Printf("kubeconfig error:%s\n", err.Error())
			fmt.Printf("Trying inClusterConfig..")
			cfg, err = rest.InClusterConfig()
			if err != nil {
				return nil, err
			}
		}
	}
	return cfg, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getDynamicClient() (dynamic.Interface, error) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	return dynamicClient, err
}

func getComposition1(kind, name, status string, compositionTree *[]CompositionTreeNode) Composition {
	var compositionString string
	fmt.Printf("Kind: %s Name: %s Composition:\n", kind, name)
	compositionString = "Kind: " + kind + " Name:" + name + " Composition:\n"
	parentComposition := Composition{}
	parentComposition.Level = 0
	parentComposition.Kind = kind
	parentComposition.Name = name
	parentComposition.Status = status
	parentComposition.Children = []Composition{}
	var root = parentComposition
	for _, compositionTreeNode := range *compositionTree {
		level := compositionTreeNode.Level
		childKind := compositionTreeNode.ChildKind
		metaDataAndOwnerReferences := compositionTreeNode.Children
		//childComposition.Children = []Composition{}
		var childrenList = []Composition{}
		for _, metaDataNode := range metaDataAndOwnerReferences {
			childComposition := Composition{}
			childName := metaDataNode.MetaDataName
			childStatus := metaDataNode.Status
			fmt.Printf("  %d %s %s\n", level, childKind, childName)
			compositionString = compositionString + " " + string(level) + " " + childKind + " " + childName + "\n"
			childComposition.Level = level
			childComposition.Kind = childKind
			childComposition.Name = childName
			childComposition.Status = childStatus
			childrenList = append(childrenList, childComposition)
		}
		root.Children = childrenList
		fmt.Printf("Root composition:%v\n", root)
		root = root.Children[0]
	}
	return parentComposition
}

// This stores Composition information in etcd accessible at the etcdServiceURL
// One option to deploy etcd is to use the CoreOS etcd-operator.
// The etcdServiceURL initialized in init() is for the example etcd cluster that
// will be created by the etcd-operator. See https://github.com/coreos/etcd-operator
//Ref:https://github.com/coreos/etcd/tree/master/client
func storeCompositions_etcd(resourceKind string, resourceName string, compositionTree *[]CompositionTreeNode) {
	//fmt.Println("Entering storeCompositions_etcd")
	jsonCompositionTree, err := json.Marshal(compositionTree)
	if err != nil {
		panic(err)
	}
	resourceComps := string(jsonCompositionTree)
	cfg := client.Config{
		//Endpoints: []string{"http://192.168.99.100:32379"},
		Endpoints: []string{etcdServiceURL},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		//HeaderTimeoutPerRequest: time.Second,
	}
	//fmt.Printf("%v\n", cfg)
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)
	// set "/foo" key with "bar" value
	//resourceKey := "/compositions/Deployment/pod42test-deployment"
	//resourceProv := "{1 ReplicaSet; 2 Pod -1}"
	resourceKey := string("/compositions/" + resourceKind + "/" + resourceName)
	fmt.Printf("Setting %s->%s\n", resourceKey, resourceComps)
	resp, err := kapi.Set(context.Background(), resourceKey, resourceComps, nil)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Set is done. Metadata is %q\n", resp)
	}
	//fmt.Printf("Getting value for %s\n", resourceKey)
	resp, err = kapi.Get(context.Background(), resourceKey, nil)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		//log.Printf("Get is done. Metadata is %q\n", resp)
		// print value
		log.Printf("%q key has %q value\n", resp.Node.Key, resp.Node.Value)
	}
	//fmt.Println("Exiting storeCompositions_etcd")
}

func (cp *ClusterCompositions) PrintCompositions() {
	cp.mux.Lock()
	defer cp.mux.Unlock()
	fmt.Println("Compositions of different Kinds in this Cluster")
	for _, compositionItem := range cp.clusterCompositions {
		kind := compositionItem.Kind
		name := compositionItem.Name
		compositionTree := compositionItem.CompositionTree
		fmt.Printf("Kind: %s Name: %s Composition:\n", kind, name)
		for _, compositionTreeNode := range *compositionTree {
			level := compositionTreeNode.Level
			childKind := compositionTreeNode.ChildKind
			metaDataAndOwnerReferences := compositionTreeNode.Children
			for _, metaDataNode := range metaDataAndOwnerReferences {
				childName := metaDataNode.MetaDataName
				childStatus := metaDataNode.Status
				fmt.Printf("  %d %s %s %s\n", level, childKind, childName, childStatus)
			}
		}
		fmt.Println("============================================")
	}
}

func printMaps() {
	fmt.Println("Printing kindVersionMap")
	for key, value := range kindVersionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing KindPluralMap")
	for key, value := range KindPluralMap {
		fmt.Printf("%s, %s\n", key, value)
	}
	fmt.Println("Printing compositionMap")
	for key, value := range compositionMap {
		fmt.Printf("%s, %s\n", key, value)
	}
}

func PrintRelatives(format string, connections []Connection) {
	switch format {
	case "flat": 
		printConnectionsFlat(connections)
	case "default":
		printConnectionsTabs(connections)
	}
}

func printConnectionsFlat(connections []Connection) {
	for _, connection := range connections {
		levelStr := strconv.Itoa(connection.Level)
		targetKind := connection.Kind
		relativeName := connection.Name
		//relType := connection.RelationType
		//relDetail := connection.RelationDetails
		//ownerDetail := connection.Owner
		//relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + ownerDetail
		relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName
		fmt.Printf(relativeEntry + "\n")
	}
}

func printConnectionsTabs(connections []Connection) {
	for _, connection := range connections {
		level := connection.Level
		for t:=1; t<level; t++ {
			fmt.Printf("\t")
		}
		fmt.Printf("%s/%s (related by: %s)\n", connection.Kind, connection.Name, connection.RelationType)
		//fmt.Printf("%s/%s (%s)\n", connection.Kind, connection.Name, connection.Owner)
	}
}

func compareConnections(c1, c2 Connection) bool {
	if c1.Kind != c2.Kind {
		return false
	} else if c1.Name != c2.Name {
		return false
	} else if c1.Namespace != c2.Namespace {
		return false
	} else if c1.Owner != c2.Owner {
		return false
	} else {
		return true
	}
}

func appendRelNames(relativesNames []string, instanceName string) []string {
	present := false
	for _, relName := range relativesNames {
		if relName == instanceName {
			present = true
			break
		}
	}
	if !present {
		relativesNames = append(relativesNames, instanceName)
	}
	return relativesNames
}

func appendRelatives(allRelatives, relatives []string) []string {
	for _, rel := range relatives {
		present := false 
		for _, existingRel := range allRelatives {
			if rel == existingRel {
				present = true
			}
		}
		if !present {
			allRelatives = append(allRelatives, rel)			
		}
	}
	return allRelatives
}

func appendConnections(allConnections, connections []Connection, level int) []Connection {
	for _, conn := range connections {
		present := false
		for j, existingConn := range allConnections {
			present = compareConnections(conn, existingConn)
			if present {
				if existingConn.Level > level {
					existingConn.Level = level
					allConnections[j] = existingConn
				}
			}
		}
		if !present {
			allConnections = append(allConnections, conn)
		}
	}
	return allConnections
}

func collectRelatives(relLeft, relRight []string) []string {
	relatives := make([]string, 0)
	for _, rel := range relLeft {
		relatives = append(relatives, rel)
	}
	for _, rel := range relRight {
		relatives = append(relatives, rel)
	}
	return relatives
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

