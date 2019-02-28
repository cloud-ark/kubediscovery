package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/coreos/etcd/client"
)

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
