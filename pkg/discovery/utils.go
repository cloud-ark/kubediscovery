package discovery

import (
	"os"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sort"
	"path/filepath"
	"github.com/coreos/etcd/client"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	cfg *rest.Config
	err error
	dynamicClient dynamic.Interface
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
	if dynamicClient == nil {
		dynamicClient, err = dynamic.NewForConfig(cfg)
	}
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
		printConnections(connections, "flat")
	case "tabbed":
		printConnectionsTabs(connections)
	case "default":
		printConnections(connections, "default")
	case "json":
		printConnectionsJSON(connections)
	}
}

func printConnectionsJSON(connections []Connection) {
	fmt.Printf("%v", connections)
}

func adjustLevels(connections []Connection) []Connection {
	for i, conni := range connections {
		for _, connj := range connections {
			// Update the level if parent-child relationship exists.
			if conni.Name == connj.OwnerName && 
			   conni.Kind == connj.OwnerKind && 
			   conni.Namespace == connj.Namespace {
			   conni.Level = connj.Level + 1
			   connections[i] = conni
			}
		}
	}
	return connections
}

func printConnections(connections []Connection, printtype string) {
	//path := make([]Connection, 0)
	var path []Connection
	pathnum := 0
	for _, connection := range connections {
		//levelStr := strconv.Itoa(connection.Level)
		if connection.Level == 1 {
			if pathnum > 0 {
				fmt.Printf("------ Path %d ------\n", pathnum)
				//printNode(connections[0], printtype, "")
				printPath(path, printtype)
				//fmt.Printf("-----------\n")
			}
			pathnum = pathnum + 1
			path = make([]Connection, 0)
			path = append(path, connections[0])
		}
		if pathnum > 0 {
			path = append(path, connection)
		}
		//relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + ownerDetail
		//relativeEntry := "Level:" + levelStr + " kind:" + connection.Kind + " name:" + connection.Name + " " + connection.Owner + " " + relType
		//fmt.Printf(relativeEntry + "\n")
	}
	fmt.Printf("------ Path %d ------\n", pathnum)
	//printNode(connections[0], printtype, "")
	printPath(path, printtype)
}

func printNode(connection Connection, printtype, relType string) {
	levelStr := strconv.Itoa(connection.Level)
	//relType := connection.RelationType
		//relativeEntry := "Level:" + levelStr + " kind:" + targetKind + " name:" + relativeName +  " related by:" + relType + " " + ownerDetail
	var relativeEntry string
	if printtype == "flat" {
		relativeEntry = "Level:" + levelStr + " kind:" + connection.Kind + " name:" + connection.Name + relType
	} else {
		relativeEntry = "Level:" + levelStr + " " + connection.Kind + "/" + connection.Name + relType
	}
	fmt.Printf(relativeEntry + "\n")
}

func printPath(connections []Connection, printtype string) {
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].Level < connections[j].Level
	})
	for i, connection := range connections {
		var relationType string
		if i > 0 {
			relationType = " [related to " + connections[i-1].Kind + "/" + connections[i-1].Name +  " by:" + connection.RelationType + "]"
		} else {
			relationType = ""
		}
		printNode(connection, printtype, relationType)
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
			// Update the level if found a shorter path.
			if present {
				if existingConn.Level > level {
					existingConn.Level = level
					allConnections[j] = existingConn
				}
			}
			if conn.Name == existingConn.OwnerName && 
			   conn.Kind == existingConn.OwnerKind && 
			   conn.Namespace == existingConn.Namespace {
			   conn.Level = existingConn.Level + 1
			   conn.RelationType = "owner reference"
			   //allConnections[j] = existingConn
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