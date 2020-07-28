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

// Composition utility functions

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

// Connection utility functions

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

func printConnections(connections []Connection, printtype string) {
	//path := make([]Connection, 0)
	var path []Connection
	//Color: https://twinnation.org/articles/35/how-to-add-colors-to-your-console-terminal-output-in-go

	pathnum := 0
	//fmt.Printf("Output Connections: %v\n", connections)
	fmt.Printf("\n::Final connections graph::\n")
	for _, connection := range connections {
		//printNode(connection, "flat", "")
		if connection.Level == 1 {
			if pathnum > 0 {
				fmt.Printf("------ Branch %d ------\n", pathnum)
				printPath(path, printtype)
			}
			pathnum = pathnum + 1
			path = make([]Connection, 0)
			path = append(path, connections[0])
		}
		if pathnum > 0 {
			path = append(path, connection)
		}
	}
	fmt.Printf("------ Branch %d ------\n", pathnum)
	printPath(path, printtype)
}

func printNode(connection Connection, printtype, relType string) {
	levelStr := strconv.Itoa(connection.Level)

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
			if connection.Peer != nil {
				relType := ""
				switch connection.RelationType {
				case relTypeLabel:
					relType = relType + green + connection.RelationType + reset
				case relTypeSpecProperty:
					relType = relType + purple + connection.RelationType + reset
				case relTypeEnvvariable:
					relType = relType + red + connection.RelationType + reset
				case relTypeAnnotation:
					relType = relType + yellow + connection.RelationType + reset
				case relTypeOwnerReference:
					relType = relType + cyan + connection.RelationType + reset
				}
				relationType = " [related to " + connection.Peer.Kind + "/" + connection.Peer.Name +  " by:" + relType + "]"
			} else {
				relationType = " [related by:" + connection.RelationType + "]"				
			}
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
	} else {
		return true
	}
}

func compareConnectionsRelType(c1, c2 Connection) bool {
	if c1.Kind != c2.Kind {
		return false
	} else if c1.Name != c2.Name {
		return false
	} else if c1.Namespace != c2.Namespace {
		return false
	} else if c1.RelationType != c2.RelationType {
		return false
	} else if c1.Peer.Name != c2.Peer.Name {
		return false
	} else {
		return true
	}
}

func AppendConnections(allConnections []Connection, connection Connection) []Connection {
	present := false
	present2 := false
	//present3 := false
	//fmt.Printf("connection.Name:%s, connection.Kind:%s\n", connection.Name, connection.Kind)
	for i, conn := range allConnections {
		//if connection.Kind == "ClusterIssuer" && (*connection.Peer).Kind == "ClusterIssuer" {
			//fmt.Printf("Conn:%v, Conn.Peer:%v, Connection:%v, Connection.Peer:%v\n",conn, *conn.Peer, connection, *connection.Peer)			
		//}
		//present = compareConnectionsRelType(conn, connection) /// working

		// Case 1: Check if connection exists without any concern with the edge type.
		// If so, store the connection as new entry towards the end; remove the current entry.
		if connection.Peer != nil {
			//fmt.Printf("conn.Kind:%s,", conn.Kind)
			//fmt.Printf("conn.Name:%s", conn.Name)
			//fmt.Printf("connection.Peer.Name:%s", connection.Peer.Name)
			//fmt.Printf("connection.Peer.Kind:%s,", connection.Peer.Kind)
			if conn.Kind == connection.Peer.Kind && conn.Name == connection.Peer.Name {
				if (*conn.Peer).Kind == connection.Kind && (*conn.Peer).Name == connection.Name {
			//fmt.Printf("Conn:%v, Conn.Peer:%v, Connection:%v, Connection.Peer:%v\n",conn, *conn.Peer, connection, *connection.Peer)
					present = true
				}
			}
		}
		if present {
			if connection.Level == 1 {
				// Store the new connection instead of existing connection; Move it at the end
				allConnectionsNew := append(allConnections[:i], allConnections[i+1:]...)
				allConnectionsNew = append(allConnectionsNew, connection)
				allConnections = allConnectionsNew
			}
			break
		}
	}

	// Case 3: Check if connection is not same as the original input
	/*originalInput := Connection{
		Kind: OrigKind,
		Name: OrigName,
		Namespace: OrigNamespace,
	}
	present = compareConnections(connection, originalInput)*/

	if !present {
		// Case 2: Check if connection has not been discovered before 
		for _, conn := range allConnections {
			present2 = compareConnectionsRelType(conn, connection)
			if present2 {
				break
			}
		}
		if !present2 {
			allConnections = append(allConnections, connection)	
		}
	} 
	return allConnections
}

func appendConnections1(allConnections, connections []Connection) []Connection {
	for _, conn := range connections {
		present := false
		for j, existingConn := range allConnections {
			present = compareConnections(conn, existingConn)
			// Update the level if found a shorter path.
			if present {
				//fmt.Printf("ExistingConn.Name:%s, ExistingConn.Level:%d, conn.Name:%s, conn.Level:%d\n", existingConn.Name, existingConn.Level, conn.Name, conn.Level)
				if existingConn.Level > conn.Level {
					existingConn.Level = conn.Level
					//existingConn.Peer = conn.Peer
					//fmt.Printf("ExistingLevel:%d, InputLevel:%d\n", existingConn.Level, level) 
					allConnections[j] = existingConn
				}
				break
			}
		}
		if !present {
			allConnections = append(allConnections, conn)
		}
	}
	return allConnections
}
